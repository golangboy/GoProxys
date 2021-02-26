package GoProxys

import "C"
import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
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

var defaultSocket5Config = &ProxyConfig{
	TimeOut:   time.Second * 60 * 10,
	SSTimeOut: time.Millisecond * 200,
	LogFile:   "socket5.log",
}

func DefaultSocket5() *Socket5Proxy {
	return &Socket5Proxy{
		Config: defaultSocket5Config,
	}
}

type Socket5Proxy struct {
	Config  *ProxyConfig
	t       *net.TCPListener
	logFile *os.File
	logger  *logrus.Logger
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
	h.t = t
	h.handleSocketListener()
}
func (h *Socket5Proxy) Close() {
	h.logger.Infoln("Stop")
	if h.logFile != nil {
		_ = h.logFile.Close()
	}
	_ = h.t.Close()
	h.logger.Exit(0)
}

type dataHelp struct {
	Data    [1500]byte
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
		Data:    [1500]byte{},
		DataLen: 20,
	}
	r.Socket.Close()
}
func (r *shadowSocket) translateServer() {
	var buff [40240]byte
	for {
		n, err := r.Socket.Read(buff[:])
		if err != nil {

			break
		}
		r.AvailTime = time.Now()

		r.ProxySocket.WriteToUDP(append(r.Header[:r.HeaderLen], buff[:n]...), &r.LocalClient)
	}
}

func (r *shadowSocket) translateClient() {
	for {
		tmp := <-r.DataChan
		r.HeaderLen = 10
		for i := 0; i < r.HeaderLen; i++ {
			r.Header[i] = tmp.Data[i]
		}
		r.AvailTime = time.Now()
		_, err := r.Socket.WriteTo(tmp.Data[r.HeaderLen:tmp.DataLen], &r.TargetServer)
		if err != nil {
			close(r.DataChan)
			break
		}

	}
}

func (h *Socket5Proxy) udpTranslate(udpProxy *net.UDPConn) {
	var buff [1500]byte
	//var client2Server = make(map[string]*net.UDPConn)
	var client2SS = make(map[string]*shadowSocket)
	//var timeOut = make(map[*net.UDPConn]time.Time)
	var m sync.Mutex
	go func() {
		for {
			m.Lock()
			//for k, v := range timeOut {
			//	if time.Now().Sub(v).Seconds() >= 2 {
			//		k.Close()
			//		delete(timeOut, k)
			//	}
			//}
			//fmt.Println("key_nums", len(client2Server))
			for k, v := range client2SS {
				if v.AvailTime.Before(time.Now()) {
					v.close()
					delete(client2SS, k)
				}
			}
			m.Unlock()
			time.Sleep(time.Second * 3)
		}
	}()

	for {
		reqLe, ClientAddr, err := udpProxy.ReadFromUDP(buff[:])
		go func(clientAddr *net.UDPAddr, recvLen int, b [1500]byte) {
			var host string
			var port string
			if err != nil || recvLen < 10 {
				h.logger.WithField("Raw", b[:reqLe]).Warningln("Udp request")
			}
			//clientData := b[10:recvLen]
			host, port = h.resolveHostPort(b[:recvLen], reqLe)
			targetServer, err := net.ResolveUDPAddr("udp4", host+":"+port)
			if err != nil {
				return
			}
			hashKey := clientAddr.String() + targetServer.String()

			//Reuse or create a ShadowSocket
			//var shadowSocket *net.UDPConn
			var ss *shadowSocket
			m.Lock()
			if client2SS[hashKey] == nil {

				newSS := shadowSocket{
					HeaderLen:    0,
					TargetServer: *targetServer,
					LocalClient:  *clientAddr,
					Socket:       newUdp(),
					DataChan:     make(chan dataHelp),
					ProxySocket:  udpProxy,
				}
				client2SS[hashKey] = &newSS
				go newSS.translateClient()
				go newSS.translateServer()
				//var newSSChan = make(chan SSData)
				//client2Chan[hashKey] = newSSChan
				//client2Server[hashKey] = shadowSocket
				//
				//go func(ss *net.UDPConn, dataChan chan SSData) {
				//	for {
				//		tmp := <-dataChan
				//		clientData := tmp.buff[10:tmp.len]
				//		ss.WriteToUDP(clientData, &tmp.targetServer)
				//
				//	}
				//}(shadowSocket, newSSChan)
			}
			ss = client2SS[hashKey]
			m.Unlock()

			ss.DataChan <- dataHelp{
				Data:    b,
				DataLen: recvLen,
			}

			//n, err = shadowSocket.WriteToUDP(clientData, targetServer)

			//Send client data to target server
			//go func(ss *net.UDPConn, bb [1500]byte) {
			//	//Maybe this target server not send data to the ss again,so need a time
			//	var tmp [1500]byte
			//	ss.SetDeadline(time.Now().Add(h.Config.SSTimeOut))
			//	ss.SetReadDeadline(time.Now().Add(h.Config.SSTimeOut))
			//	n, err = ss.Read(tmp[:])
			//	udpProxy.WriteToUDP(append(bb[:10], tmp[:n]...), ClientAddr)
			//	if err != nil {
			//		m.Lock()
			//		delete(client2Server, ClientAddr.String())
			//		m.Unlock()
			//		shadowSocket.Close()
			//	}
			//}(shadowSocket, b)

		}(ClientAddr, reqLe, buff)
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
	udpProxyServer := newUdp()
	_, localUdpPort, _ := net.SplitHostPort(udpProxyServer.LocalAddr().String())
	shortPort := uint16(cast.ToInt16(localUdpPort))
	go h.udpTranslate(udpProxyServer)

	for {
		client, err := h.t.Accept()
		if err != nil {
			panic(err)
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
					server, err := net.Dial("tcp", net.JoinHostPort(host, port))
					if err != nil {
						h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Dial target failed")
						return
					}
					_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
					if err != nil {
						h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Response target failed")
						return
					}
					server.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
					server.SetDeadline(time.Now().Add(h.Config.TimeOut))
					conn.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
					conn.SetDeadline(time.Now().Add(h.Config.TimeOut))

					go io.Copy(server, conn)
					go io.Copy(conn, server)
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
