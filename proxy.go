package GoProxys

import "time"

type ProxyConfig struct {
	TCPTimeOut    time.Duration
	UDPTimeOut    time.Duration //UDP Shadown Socket Recv TCPTimeOut
	LogFile       string
	EnableLogFile bool
}
