package GoProxys

import "time"

type ProxyConfig struct {
	TimeOut       time.Duration
	UDPTimeOut    time.Duration //UDP Shadown Socket Recv TimeOut
	LogFile       string
	EnableLogFile bool
}
