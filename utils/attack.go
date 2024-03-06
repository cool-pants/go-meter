package gogeta

import (
	"math"
	"net"
)

var (
	DefaultLocalAddr             = net.IPAddr{IP: net.IPv4zero}
	DefaultConnections           = 10000
	DefaultMaxConnections        = 0
	DefaultWorkers        uint64 = 10
	DefaultMaxWorkers     uint64 = math.MaxUint64
)
