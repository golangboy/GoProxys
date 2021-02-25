package GoProxys

import (
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"os"
	"strconv"
	"time"
)

var defaultSocket5Config = &ProxyConfig{
	TimeOut: time.Second * 60 * 10,
	LogFile: "socket5.log",
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
	h.handleTCPListener()
}
func (h *Socket5Proxy) Close() {
	h.logger.Infoln("Stop")
	if h.logFile != nil {
		h.logFile.Close()
	}
	h.t.Close()
	h.logger.Exit(0)
}
func (h *Socket5Proxy) handleTCPListener() {
	for {
		client, err := h.t.Accept()
		if err != nil {
			panic(err)
		}

		go func() {
			var b [1024]byte
			n, err := client.Read(b[:])
			if err != nil {
				h.logger.WithError(err).Infoln("Read hello handshake")
				return
			}
			if b[0] == 0x05 {
				client.Write([]byte{0x05, 0x00})
				n, err = client.Read(b[:])
				var host, port string
				switch b[3] {
				case 0x01: //IP V4
					host = net.IPv4(b[4], b[5], b[6], b[7]).String()
				case 0x03: //Domain
					host = string(b[5 : n-2])
				case 0x04: //IP V6
					host = net.IP{b[4], b[5], b[6], b[7], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15], b[16], b[17], b[18], b[19]}.String()
				}
				port = strconv.Itoa(int(b[n-2])<<8 | int(b[n-1]))

				server, err := net.Dial("tcp", net.JoinHostPort(host, port))
				if err != nil {
					h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Dial target failed")
					return
				}
				_, err = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
				if err != nil {
					h.logger.WithField("target", host+":"+port).WithError(err).Warningln("Response target failed")
					return
				}
				server.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
				server.SetDeadline(time.Now().Add(h.Config.TimeOut))
				client.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
				client.SetDeadline(time.Now().Add(h.Config.TimeOut))

				go io.Copy(server, client)
				go io.Copy(client, server)
			}
		}()
	}
}
