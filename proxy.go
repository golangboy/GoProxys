package GoProxys

import "time"

type ProxyConfig struct {
	TimeOut       time.Duration
	LogFile       string
	EnableLogFile bool
}
