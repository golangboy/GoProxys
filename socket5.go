package GoProxys

import (
	"bytes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	CmdNone = iota
	CmdConnect
	CmdBind
	CmdUdp
)

type UDPCallBack func(send bool, sender string, receiver string, data []byte, dataLen int)
type TCPCallBack func(send bool, sender string, receiver string, data []byte, dataLen int)

var defaultSocket5Config = &ProxyConfig{
	TCPTimeOut: time.Second * 30,
	UDPTimeOut: time.Second * 3,
	LogFile:    "socket5.log",
}

func DefaultSocket5() *Socket5Proxy {
	return &Socket5Proxy{
		Config: defaultSocket5Config,
	}
}

type Socket5Proxy struct {
	Config         *ProxyConfig
	tcpProxyServer *net.TCPListener
	logFile        *os.File
	logger         *logrus.Logger
	client2SS      map[string]*shadowSocket //UDP
	m              sync.Mutex

	udpProxyServer *net.UDPConn

	//ssh client
	SSHClient *ssh.Client

	//callback
	UdpCallBack UDPCallBack
	TcpCallBack TCPCallBack
}

func (h *Socket5Proxy) RunSocket5Proxy(addr *net.TCPAddr) {
	t, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		panic(addr.String() + " already in use")
	}
	h.logger = logrus.New()
	if h.Config.EnableLogFile == false {
		h.logger.SetOutput(os.Stdout)

	} else {
		h.logFile, _ = os.Create(h.Config.LogFile)
		h.logger.SetOutput(h.logFile)
	}

	h.logger.SetReportCaller(true)
	h.logger.Infoln("Start")
	h.tcpProxyServer = t
	h.handleSocketListener()
}
func (h *Socket5Proxy) Close() {
	h.logger.Infoln("Stop")

	//Close Udp showdown socket
	h.m.Lock()
	for _, v := range h.client2SS {
		v.close()
	}
	h.client2SS = nil
	h.m.Unlock()

	//Close Udp proxy socket

	h.udpProxyServer.Close()

	if h.logFile != nil {
		_ = h.logFile.Close()
	}
	//SSH Client
	if h.SSHClient != nil {
		h.SSHClient.Close()
	}

	//Close Tcp
	_ = h.tcpProxyServer.Close()

	//Close Log File
	h.logFile.Close()

	h.logger = nil
}

type dataHelp struct {
	Data    *bytes.Buffer
	DataLen int
}

type shadowSocket struct {
	Header       [10]byte
	HeaderLen    int
	TargetServer net.UDPAddr
	LocalClient  net.UDPAddr
	Socket       *net.UDPConn
	DataChan     chan dataHelp
	ProxySocket  *net.UDPConn
	AvailTime    time.Time
}

func (r *shadowSocket) close() {
	r.DataChan <- dataHelp{
		Data:    nil,
		DataLen: 20,
	}
	r.Socket.Close()
}
func (r *shadowSocket) translateServer(udpCallBack UDPCallBack) {
	var buff [65535]byte
	for {
		n, addr, err := r.Socket.ReadFromUDP(buff[:])
		if err != nil {

			break
		}
		r.AvailTime = time.Now()

		if udpCallBack != nil {
			udpCallBack(false, addr.String(), r.LocalClient.String(), buff[:n], n)
		}
		r.ProxySocket.WriteToUDP(append(r.Header[:r.HeaderLen], buff[:n]...), &r.LocalClient)
	}
}

func (r *shadowSocket) translateClient(udpCallBack UDPCallBack) {
	for {
		tmp := <-r.DataChan
		if tmp.Data == nil {
			close(r.DataChan)
			break
		}
		switch tmp.Data.Bytes()[3] {
		case 1:
			//ipv4
			r.HeaderLen = 3 + 1 + 4 + 2
			break
		case 3:
			//domain
			r.HeaderLen = int(3 + 1 + tmp.Data.Bytes()[4] + 2)
			break
		case 4:
			//ipv6
			r.HeaderLen = 3 + 1 + 16 + 2
			break

		}
		for i := 0; i < r.HeaderLen; i++ {
			r.Header[i] = tmp.Data.Bytes()[i]
		}
		r.AvailTime = time.Now()
		if udpCallBack != nil {
			udpCallBack(true, r.LocalClient.String(), r.TargetServer.String(), tmp.Data.Bytes(), tmp.DataLen)
		}
		_, err := r.Socket.WriteTo(tmp.Data.Bytes()[r.HeaderLen:tmp.DataLen], &r.TargetServer)
		if err != nil {
			close(r.DataChan)
			break
		}

	}
}

func (h *Socket5Proxy) udpTranslate(udpProxy *net.UDPConn) {
	var buff [65535]byte
	h.client2SS = make(map[string]*shadowSocket)

	//Remove unused sockets
	go func() {
		for {
			var closed bool
			h.m.Lock()
			for k, v := range h.client2SS {
				if v.AvailTime.Add(h.Config.UDPTimeOut).Before(time.Now()) {
					v.close()
					delete(h.client2SS, k)
				}
			}
			closed = h.client2SS == nil
			h.m.Unlock()
			if closed {
				break
			}
			time.Sleep(time.Second * 1)
		}
	}()

	for {
		reqLe, ClientAddr, err := udpProxy.ReadFromUDP(buff[:])
		if err != nil {
			break
		}

		newBf := bytes.NewBuffer(nil)
		io.Copy(newBf, bytes.NewReader(buff[:reqLe]))
		go func(clientAddr *net.UDPAddr, recvLen int, b *bytes.Buffer) {
			var host string
			var port string
			if err != nil || recvLen < 10 {
				h.logger.WithField("Raw", b.Bytes()[:reqLe]).Warningln("Udp request")
			}
			host, port = h.resolveHostPort(b.Bytes()[:recvLen], reqLe)
			targetServer, err := net.ResolveUDPAddr("udp4", host+":"+port)
			if err != nil {
				return
			}
			hashKey := clientAddr.String() + targetServer.String()

			//Reuse or create a ShadowSocket
			var ss *shadowSocket
			h.m.Lock()
			if h.client2SS[hashKey] == nil {
				newSS := shadowSocket{
					HeaderLen:    0,
					TargetServer: *targetServer,
					LocalClient:  *clientAddr,
					Socket:       newUdp(),
					DataChan:     make(chan dataHelp),
					ProxySocket:  udpProxy,
				}
				h.client2SS[hashKey] = &newSS
				go newSS.translateClient(h.UdpCallBack)
				go newSS.translateServer(h.UdpCallBack)
			}
			ss = h.client2SS[hashKey]
			h.m.Unlock()

			ss.DataChan <- dataHelp{
				Data:    b,
				DataLen: recvLen,
			}

		}(ClientAddr, reqLe, newBf)
	}
}

func (h *Socket5Proxy) resolveHostPort(b []byte, n int) (string, string) {
	var host, port string
	var offset byte
	switch b[3] {
	case 0x01: //IP V4
		offset = 4
		host = net.IPv4(b[4], b[5], b[6], b[7]).String()
	case 0x03: //Domain
		if n-2 <= 5 {
			h.logger.WithField("Raw", b[:n]).Warningln("Parse  domain error")
			return "", ""
		}
		offset = b[4] + 1
		host = string(b[5 : n-2])
	case 0x04: //IP V6
		if n < 20 {
			h.logger.WithField("Raw", b[:n]).Warningln("Parse ipv6 addr error")
			return "", ""
		}
		offset = 16
		host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
	}
	port = strconv.Itoa(int(b[4+offset])<<8 | int(b[5+offset]))
	return host, port
}

func (h *Socket5Proxy) handleSocketListener() {
	//
	h.udpProxyServer = newUdp()
	_, localUdpPort, _ := net.SplitHostPort(h.udpProxyServer.LocalAddr().String())
	shortPort := uint16(cast.ToInt16(localUdpPort))
	go h.udpTranslate(h.udpProxyServer)

	for {
		client, err := h.tcpProxyServer.Accept()
		if err != nil {
			h.tcpProxyServer.Close()
			return
		}
		go func(conn net.Conn) {
			var b [10240]byte
			n, err := conn.Read(b[:])
			if err != nil {
				h.logger.WithError(err).Infoln("Read hello handshake")
				return
			}
			//version 5
			if b[0] == 0x05 {
				//No Auth
				conn.Write([]byte{0x05, 0x00})
				n, err = conn.Read(b[:])
				if err != nil {
					conn.Close()
					return
				}
				var host, port string
				var cmd byte
				cmd = b[1]
				host, port = h.resolveHostPort(b[:n], n)
				if cmd == CmdConnect {
					var server net.Conn
					if h.SSHClient != nil {
						server, err = h.SSHClient.Dial("tcp", net.JoinHostPort(host, port))
					} else {
						server, err = net.Dial("tcp", net.JoinHostPort(host, port))
					}
					if err != nil {
						h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Dial target failed")
						return
					}
					_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
					if err != nil {
						h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Response target failed")
						return
					}
					go func() {
						for {
							var buff [10240]byte
							conn.SetReadDeadline(time.Now().Add(h.Config.TCPTimeOut))
							n, err := conn.Read(buff[:])
							if err != nil {
								break
							}
							if h.TcpCallBack != nil {
								h.TcpCallBack(true, conn.RemoteAddr().String(), server.RemoteAddr().String(), buff[:n], n)
							}
							server.Write(buff[:n])
						}
					}()
					go func() {
						for {
							var buff [10240]byte
							server.SetReadDeadline(time.Now().Add(h.Config.TCPTimeOut))
							n, err := server.Read(buff[:])
							if err != nil {
								break
							}
							if h.TcpCallBack != nil {
								h.TcpCallBack(false, server.RemoteAddr().String(), conn.RemoteAddr().String(), buff[:n], n)
							}
							conn.Write(buff[:n])
						}
					}()
					//go io.Copy(server, conn)
					//go io.Copy(conn, server)
				} else if cmd == CmdUdp {
					defer conn.Close()
					//Response Success
					n, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, byte(shortPort >> 8), byte(shortPort << 8 >> 8)}) //响应客户端连接成功
					if err != nil {
						return
					}
					//This tcp connection will not be use to translate data
				} else {
					h.logger.WithField("cmd", cmd).Warningln("Unknown cmd")
					conn.Close()
				}
			} else {
				h.logger.WithField("Raw", b[:n]).Warningln("Unknown version")
				conn.Close()
			}
		}(client)
	}
}
