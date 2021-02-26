package GoProxys

import "time"

type ProxyConfig struct {
	TimeOut       time.Duration
	SSTimeOut     time.Duration //UDP Shadown Socket Recv TimeOut
	LogFile       string
	EnableLogFile bool
}
