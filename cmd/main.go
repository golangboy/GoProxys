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
	go func() {
		for {
			time.Sleep(time.Second)
			fmt.Println("cur", GoProxys.GetSocketsNum())
		}
	}()
	//a, _ := net.ResolveTCPAddr("tcp4", ":7777")
	//d := pkg.DefaultHttp()
	//go d.RunHttpProxy(a)
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	g := GoProxys.DefaultSocket5()
	g.Config.EnableLogFile = true
	go g.RunSocket5Proxy(b)
	//time.Sleep(time.Second * 60)
	//var zz int
	//fmt.Scan(&zz)
	//g.Close()
	//time.Sleep(time.Second * 100)
}
