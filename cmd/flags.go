package commands

import (
	"fmt"
	gogeta "github.com/cool-pants/gogeta/utils"
	"net"
	"strconv"
	"strings"
	"time"
)

/*
List of all the Flags to be supported
name => name of the attack
target => File path to be loaded
format => format of the target file
rate => Rate of the requests to be sent, need to be configured with a Pacer
workers => Number of Virtual Users to be used
maxWorkers => Maximum number of Virtual Users that can be used
workerRamp => By how much to increase the number of workers (workers start at 0 by default)
connections => Max open idle connections per target host
maxConnections => Max connections per target host
laddr => Local IP Address (will send http request)
keepalive => Use persistent connections (True by Default)
output => Output file path, stdout by default
*/

type localAddr struct{ *net.IPAddr }

func (ip *localAddr) Type() string {
	return "LocalIPAddress"
}

func (ip *localAddr) Set(value string) (err error) {
	ip.IPAddr, err = net.ResolveIPAddr("ip", value)
	return
}

type rateFlag struct{ *gogeta.Rate }

func (f *rateFlag) Type() string {
	return "Rate"
}

func (f *rateFlag) Set(v string) (err error) {
	if v == "infinity" {
		return nil
	}

	ps := strings.SplitN(v, "/", 2)
	switch len(ps) {
	case 1:
		ps = append(ps, "1s")
	case 0:
		return fmt.Errorf("-rate format %q doesn't match the \"freq/duration\" format (i.e. 50/1s)", v)
	}

	f.Freq, err = strconv.Atoi(ps[0])
	if err != nil {
		return err
	}

	if f.Freq == 0 {
		return nil
	}

	switch ps[1] {
	case "ns", "us", "µs", "ms", "s", "m", "h":
		ps[1] = "1" + ps[1]
	}

	f.Per, err = time.ParseDuration(ps[1])
	return err
}

func (f *rateFlag) String() string {
	if f.Rate == nil {
		return ""
	}
	return fmt.Sprintf("%d/%s", f.Freq, f.Per)
}
