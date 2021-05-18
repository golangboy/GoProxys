package GoProxys

import (
	"fmt"
	"github.com/spf13/cast"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func readStringUntil(reader io.Reader, token string) (string, bool) {
	var t [1]byte
	var pos int
	var tmp string
	for {
		_, err := reader.Read(t[:])
		tmp = tmp + string(t[:])
		if err != nil {
			return "", false
		}
		if t[0] == token[pos] {
			pos = pos + 1
		} else {
			pos = 0
		}
		if pos == len(token) {
			break
		}

	}
	return tmp[:len(tmp)-len(token)], true
}

func getHttpKey(s string) string {
	r := strings.Split(s, ": ")
	if len(r) != 2 {
		return ""
	}
	return r[0]
}
func getHttpValue(s string) string {
	r := strings.Split(s, ": ")
	if len(r) != 2 {
		return ""
	}
	return r[1]
}

func newUdp() *net.UDPConn {
	a, _ := net.ResolveUDPAddr("udp", ":")
	c, _ := net.ListenUDP("udp", a)
	return c
}

var foreignAddr string

func IsTargetLocal(host string) bool {
	type Foo struct {
		Cid   string `json:"cid"`
		Cip   string `json:"cip"`
		Cname string `json:"cname"`
	}
	if host == "127.0.0.1" || host == "localhost" {
		return true
	}
	addr, _ := net.InterfaceAddrs()
	for _, v := range addr {
		if strings.Index(v.String(), host) >= 0 {
			return true
		}
	}

	if len(foreignAddr) == 0 {
		resp, _ := http.Get("http://whatismyip.akamai.com/")
		if resp != nil {
			bs, err := io.ReadAll(resp.Body)
			fmt.Println(err)
			defer resp.Body.Close()
			str := string(bs)
			foreignAddr = str
			fmt.Println(foreignAddr)
		}
	}
	return foreignAddr == host
}

func GetSocketsNum() int {
	var args string
	if runtime.GOOS == "windows" {
		args = "-ano"
	} else {
		args = "-anp"
	}
	z := exec.Command(`netstat`, args)
	m, _ := z.StdoutPipe()
	z.Start()
	bbs, _ := ioutil.ReadAll(m)
	ret := string(bbs)
	sps := strings.Split(ret, cast.ToString(os.Getpid()))
	return len(sps) - 1
}

func StartWatch() {
	go func() {
		for {
			fmt.Println("Goroutine:", runtime.NumGoroutine(), " ", "Sockets:", GetSocketsNum(), " ", "Pid", os.Getpid())
			time.Sleep(time.Second * 3)
		}
	}()
}
