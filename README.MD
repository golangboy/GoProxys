# GoProxys

## TODO

> Provide a Simple Proxys Server

## Support Proxy Protocol

- Http
- Https
- Socket5(TCP + UDP)

#Tips
> Sockets-UDP only support IPV4 , so you need set the flag of resolve domain local when using the proxy sofaware. 

## Demo

```go
package main

import (
	"github.com/blacknight2018/GoProxys"
	"net"
)

func main() {
	addr, _ := net.ResolveTCPAddr("tcp", ":8888")
	s := GoProxys.DefaultSocket5()
	s.RunSocket5Proxy(addr)
}
```