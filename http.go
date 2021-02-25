package GoProxys

import (
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var defaultHttpConfig = &ProxyConfig{
	TimeOut: time.Second * 60 * 10,
	LogFile: "http.log",
}

func DefaultHttp() *HttpProxy {
	return &HttpProxy{
		Config: defaultHttpConfig,
	}
}

type HttpProxy struct {
	Config  *ProxyConfig
	t       *net.TCPListener
	logFile *os.File
	logger  *logrus.Logger
}

// Start a http/s Proxy Server
func (h *HttpProxy) RunHttpProxy(addr *net.TCPAddr) {
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

func (h *HttpProxy) Close() {
	h.logger.Infoln("Stop")
	if h.logFile != nil {
		h.logFile.Close()
	}
	h.t.Close()
	h.logger.Exit(0)
}

type httpReqHeader struct {
	Method  string
	Url     string
	Version string
	Headers []string
	Raw     string
	Body    string
}

func parseHttpRequest(reader io.Reader) (httpReqHeader, bool) {
	var ret httpReqHeader
	var buff = make([]byte, 1024)
	method, ok := readStringUntil(reader, " ")
	if !ok {
		reader.Read(buff[:])
		ret.Raw = ret.Raw + string(buff[:])
		return ret, false
	}
	ret.Method = method
	ret.Raw = ret.Raw + ret.Method + " "

	ret.Raw = ret.Raw + " "
	u, ok := readStringUntil(reader, " ")
	if !ok {
		reader.Read(buff[:])
		ret.Raw = ret.Raw + string(buff[:])
		return ret, false
	}
	ret.Raw = ret.Raw + u + " "
	ret.Url = u
	v, ok := readStringUntil(reader, "\r\n")
	if !ok {
		reader.Read(buff[:])
		ret.Raw = ret.Raw + string(buff[:])
		return ret, false
	}
	ret.Raw = ret.Raw + v + "\r\n"
	ret.Version = v
	for {
		head, ok := readStringUntil(reader, "\r\n")
		ret.Raw = ret.Raw + head + "\r\n"
		if !ok {
			break
		}
		if len(head) == 0 {
			break
		}
		ret.Headers = append(ret.Headers, head)
	}
	return ret, true
}

func (h *HttpProxy) handleTCPListener() {
	for {
		c, err := h.t.Accept()
		if err == nil {
			go func(c net.Conn) {
				reqHeader, ok := parseHttpRequest(c)
				if !ok {
					h.logger.WithField("Raw", reqHeader.Raw).Warningln("parse request failed")
					c.Close()
					return
				}
				var Host string
				for _, v := range reqHeader.Headers {
					if strings.ToLower(getHttpKey(v)) == "host" {
						Host = getHttpValue(v)
						if strings.Index(Host, ":") == -1 {
							Host = Host + ":80"
						}
						break
					}
				}
				tcpAddr, _ := net.ResolveTCPAddr("tcp4", Host)
				r, err := net.DialTCP("tcp4", nil, tcpAddr)
				if err != nil {
					h.logger.WithField("Host", Host).WithError(err).Println("Dial target host failed")
					return
				}
				r.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
				r.SetDeadline(time.Now().Add(h.Config.TimeOut))
				c.SetReadDeadline(time.Now().Add(h.Config.TimeOut))
				c.SetDeadline(time.Now().Add(h.Config.TimeOut))

				//HTTPS Proxy
				if reqHeader.Method == http.MethodConnect {
					c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
					go func() {
						io.Copy(r, c)
					}()
					go func() {
						io.Copy(c, r)
					}()
				} else {
					r.Write([]byte(reqHeader.Raw))
					go func() {
						io.Copy(r, c)
					}()
					go func() {
						io.Copy(c, r)
					}()
				}

			}(c)
		} else {
			h.logger.WithError(err).Warningln("Accept")
		}
	}
}
