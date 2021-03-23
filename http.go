package GoProxys

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type HttpCallBack func(send bool, data []byte) []byte
type HttpConnect func(conn net.Conn, host string, port string)

var defaultHttpConfig = &ProxyConfig{
	TCPTimeOut: time.Second * 60,
	LogFile:    "http.log",
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

	HttpCallBack HttpCallBack
	HttpConnect  HttpConnect
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
	Body    []byte
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

	u, ok := readStringUntil(reader, " ")

	//http://xxx.xxx/abc -> /abc
	if len(u) >= 7 && u[:4] == "http" {
		u = u[7:]
		pos := strings.Index(u, "/")
		if pos != -1 {
			u = u[pos:]
		}
	}
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
	var needRead int
	for {
		head, ok := readStringUntil(reader, "\r\n")
		ret.Raw = ret.Raw + head + "\r\n"
		if !ok {
			break
		}
		if len(head) == 0 {
			break
		}
		if strings.ToLower(getHttpKey(head)) == "content-length" {
			tmp := getHttpValue(head)
			needRead, _ = strconv.Atoi(tmp)
		}
		ret.Headers = append(ret.Headers, head)
	}
	for needRead > 0 {
		var buff [1024]byte
		var n int
		var err error
		if needRead > 1024 {
			n, err = reader.Read(buff[:])
			needRead = needRead - 1024
		} else {
			n, err = reader.Read(buff[:needRead])
			needRead = needRead - n
		}
		if err != nil {
			break
		}
		ret.Body = append(ret.Body, buff[:n]...)
		if needRead == 0 {
			break
		}
	}
	return ret, true
}

func ioCopyWithTimeOut(dst net.Conn, src net.Conn, timeOut time.Duration, f func(data []byte) []byte) error {
	var buff [10240]byte
	for {
		src.SetReadDeadline(time.Now().Add(timeOut))
		n, err := src.Read(buff[:])
		if err != nil {
			return err
		}
		if f != nil {
			ret := f(buff[:n])
			dst.Write(ret)
		} else {
			dst.Write(buff[:n])
		}
	}
	return nil
}

func (h *HttpProxy) handleTCPListener() {
	for {
		c, err := h.t.Accept()
		if err == nil && c != nil {
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
				if h.HttpConnect != nil {
					pos := strings.Index(Host, ":")
					if pos == -1 {
						fmt.Println(Host)
						c.Close()
						return
					}
					targetAddr := Host[:pos]
					targetPort := Host[pos+1:]
					if reqHeader.Method == http.MethodConnect {
						c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
						h.HttpConnect(c, targetAddr, targetPort)
						return
					}
				}
				tcpAddr, _ := net.ResolveTCPAddr("tcp4", Host)
				r, err := net.DialTCP("tcp4", nil, tcpAddr)
				if err != nil {
					h.logger.WithField("Host", Host).WithError(err).Println("Dial target host failed")
					return
				}

				//HTTPS Proxy
				if reqHeader.Method == http.MethodConnect {
					c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
					go func() {
						ioCopyWithTimeOut(r, c, h.Config.TCPTimeOut, nil)
					}()
					go func() {
						ioCopyWithTimeOut(c, r, h.Config.TCPTimeOut, nil)
					}()
				} else {
					r.Write([]byte(reqHeader.Raw))
					go func() {

						//Recv From Client
						ioCopyWithTimeOut(r, c, h.Config.TCPTimeOut, func(data []byte) []byte {
							if h.HttpCallBack != nil {
								return h.HttpCallBack(true, data)
							}
							return data
						})
					}()
					go func() {

						//Recv From Server
						ioCopyWithTimeOut(c, r, h.Config.TCPTimeOut, func(data []byte) []byte {
							if h.HttpCallBack != nil {
								return h.HttpCallBack(false, data)
							}
							return data
						})
					}()
				}

			}(c)
		} else {
			h.logger.WithError(err).Warningln("Accept")
		}
	}
}
