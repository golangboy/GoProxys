# GoProxys

## TODO

> Provide a Simple Proxys Server

## Support Proxy Protocol

- Http
- Https
- Socket5(TCP + UDP)

## Tips

> No Authentication

# How to use

### Socket5 Server

> Start a Socket5 Server

```go
package main

import (
	"github.com/blacknight2018/GoProxys"
	"net"
)

func main() {
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	s := GoProxys.DefaultSocket5()
	s.RunSocket5Proxy(b)
}
```

### CallBack

> If you want to watch the tcp/udp data

```go
package main

import (
	"fmt"
	"github.com/blacknight2018/GoProxys"
	"net"
)

func main() {
	b, _ := net.ResolveTCPAddr("tcp4", ":8888")
	s := GoProxys.DefaultSocket5()
	s.TcpCallBack = func(send bool, sender string, receiver string, data []byte, dataLen int) {
		fmt.Println("tcp", sender)
	}
	s.UdpCallBack = func(send bool, sender string, receiver string, data []byte, dataLen int) {
		fmt.Println("udp", sender)
	}
	s.RunSocket5Proxy(b)
}

```

### HTTP Proxy

> Support Https

```go
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

```

# Author

> You can contact with me by wechat
>
![avatar](images/contact.jpg)