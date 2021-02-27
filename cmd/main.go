package main

import (
	"github.com/blacknight2018/GoProxys"
	"net"
)

func main() {
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	h := GoProxys.DefaultHttp()
	h.RunHttpProxy(b)
}
