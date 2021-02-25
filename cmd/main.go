package main

import (
	"fmt"
	"github.com/blacknight2018/GoProxys"
	"net"
	"runtime"
	"time"
)

func main() {
	go func() {
		for {
			fmt.Println(runtime.NumGoroutine())
			time.Sleep(time.Second)
		}
	}()
	//a, _ := net.ResolveTCPAddr("tcp4", ":7777")
	//d := pkg.DefaultHttp()
	//go d.RunHttpProxy(a)
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	g := GoProxys.DefaultSocket5()
	g.Config.EnableLogFile = true
	g.RunSocket5Proxy(b)
}
