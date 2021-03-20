package main

import (
	"github.com/blacknight2018/GoProxys"
	"io"
	"net"
)

func main() {
	go GoProxys.StartWatch()
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	s := GoProxys.DefaultSocket5()
	s.TcpConnect = func(conn net.Conn, host string, port string) {
		c, err := net.Dial("tcp4", host+":"+port)
		if err == nil {
			go io.Copy(conn, c)
			io.Copy(c, conn)
			c.Close()
		}
	}
	s.RunSocket5Proxy(b)
}
