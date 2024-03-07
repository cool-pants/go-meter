package gogeta

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	DefaultLocalAddr             = net.IPAddr{IP: net.IPv4zero}
	DefaultConnections           = 10000
	DefaultMaxConnections        = 0
	DefaultWorkers        uint64 = 10
	DefaultMaxWorkers     uint64 = math.MaxUint64
	DefaultTimeout               = 30 * time.Second
)

type Attacker struct {
	dialer     *net.Dialer
	client     http.Client
	stopch     chan struct{}
	stopOnce   sync.Once
	workers    uint64
	maxWorkers uint64
}

func LocalAddr(addr net.IPAddr) func(*Attacker) {
	return func(a *Attacker) {
		tr := a.client.Transport.(*http.Transport)
		a.dialer.LocalAddr = &net.TCPAddr{IP: addr.IP, Zone: addr.Zone}
		tr.DialContext = a.dialer.DialContext
	}
}

func Workers(w uint64) func(*Attacker) {
	return func(a *Attacker) {
		a.workers = w
	}
}

func MaxWorkers(w uint64) func(*Attacker) {
	return func(a *Attacker) {
		a.workers = w
	}
}

func KeepAlive(keepalive bool) func(*Attacker) {
	return func(a *Attacker) {
		tr := a.client.Transport.(*http.Transport)
		tr.DisableKeepAlives = !keepalive
		if !keepalive {
			a.dialer.KeepAlive = 0
			tr.DialContext = a.dialer.DialContext
		}
	}
}

func Connections(connections int) func(*Attacker) {
	return func(a *Attacker) {
		tr := a.client.Transport.(*http.Transport)
		tr.MaxIdleConnsPerHost = connections
	}
}

func MaxConnections(maxConnections int) func(*Attacker) {
	return func(a *Attacker) {
		tr := a.client.Transport.(*http.Transport)
		tr.MaxConnsPerHost = maxConnections
	}
}

func NewAttacker(opts ...func(a *Attacker)) *Attacker {
	a := &Attacker{
		stopch:     make(chan struct{}),
		stopOnce:   sync.Once{},
		workers:    DefaultWorkers,
		maxWorkers: DefaultMaxWorkers,
	}

	a.dialer = &net.Dialer{
		LocalAddr: &net.TCPAddr{IP: DefaultLocalAddr.IP, Zone: DefaultLocalAddr.Zone},
		KeepAlive: 30 * time.Second,
	}

	a.client = http.Client{
		Timeout: DefaultTimeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         a.dialer.DialContext,
			MaxIdleConnsPerHost: DefaultConnections,
			MaxConnsPerHost:     DefaultMaxConnections,
		},
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

func (a *Attacker) Attack(tr Targeter, p Pacer, du time.Duration, name string) <-chan *Result {
	var wg sync.WaitGroup

	workers := a.workers
	if workers > a.maxWorkers {
		workers = a.maxWorkers
	}

	atk := &attack{
		name:  name,
		began: time.Now(),
	}

	results := make(chan *Result)
	ticks := make(chan struct{})
	for i := uint64(0); i < workers; i++ {
		wg.Add(1)
		go a.attack(tr, atk, &wg, ticks, results)
	}

	go func() {
		defer func() {
			close(ticks)
			wg.Wait()
			close(results)
			a.Stop()
		}()

		count := uint64(0)
		for {
			elapsed := time.Since(atk.began)
			if du > 0 && elapsed > du {
				return
			}

			wait, stop := p.Pace(elapsed, count)
			if stop {
				return
			}

			time.Sleep(wait)

			if workers < a.maxWorkers {
				select {
				case ticks <- struct{}{}:
					count++
					continue
				case <-a.stopch:
					return
				default:
					// all workers are blocked. start one more and try again
					workers++
					wg.Add(1)
					go a.attack(tr, atk, &wg, ticks, results)
				}
			}

			select {
			case ticks <- struct{}{}:
				count++
			case <-a.stopch:
				return
			}
		}
	}()

	return results
}

func (a *Attacker) Stop() bool {
	select {
	case <-a.stopch:
		return false
	default:
		a.stopOnce.Do(func() { close(a.stopch) })
		return true
	}
}

type attack struct {
	name  string
	began time.Time

	seqmu sync.Mutex
	seq   uint64
}

func (a *Attacker) attack(tr Targeter, atk *attack, workers *sync.WaitGroup, ticks <-chan struct{}, results chan<- *Result) {
	defer workers.Done()
	for range ticks {
		results <- a.hit(tr, atk)
	}
}

func (a *Attacker) hit(tr Targeter, atk *attack) *Result {
	var (
		res         = Result{Attack: atk.name}
		tgt *Target = &Target{}
		err error
	)

	//
	// Subtleness ahead! We need to compute the result timestamp in
	// the same critical section that protects the increment of the sequence
	// number because we want the same total ordering of timestamps and sequence
	// numbers. That is, we wouldn't want two results A and B where A.seq > B.seq
	// but A.timestamp < B.timestamp.
	//
	// Additionally, we calculate the result timestamp based on the same beginning
	// timestamp using the Add method, which will use monotonic time calculations.
	//
	atk.seqmu.Lock()
	res.Timestamp = atk.began.Add(time.Since(atk.began))
	res.Seq = atk.seq
	atk.seq++
	atk.seqmu.Unlock()

	defer func() {
		res.Latency = time.Since(res.Timestamp)
		if err != nil {
			res.Error = err.Error()
		}
	}()

	/*This is where I have to implement the chained Requests Magic*/

	var cache = make(map[string]string)

	for {
		if err = tr(tgt); err != nil {
			a.Stop()
			return &res
		}

		res.Method = tgt.Method
		res.URL = tgt.URL

		req, err := tgt.Request(cache)
		if err != nil {
			return &res
		}

		if atk.name != "" {
			req.Header.Set("X-Gogeta-Attack", atk.name)
		}

		req.Header.Set("X-Gogeta-Seq", strconv.FormatUint(res.Seq, 10))

		r, err := a.client.Do(req)
		if err != nil {
			return &res
		}
		defer r.Body.Close()

		body := io.Reader(r.Body)

		if res.Body, err = io.ReadAll(body); err != nil {
			return &res
		} else if _, err = io.Copy(io.Discard, r.Body); err != nil {
			return &res
		}

		resBody := make(map[string]string)

		err = json.Unmarshal(res.Body, &resBody)
		handleErrors(err, fmt.Sprintf("Failure in Unmarshalling Response %v", err))

		cache["txn_uuid"] = resBody["id"]

		res.BytesIn = uint64(len(res.Body))

		if req.ContentLength != -1 {
			res.BytesOut = uint64(req.ContentLength)
		}

		if res.Code = uint16(r.StatusCode); res.Code < 200 || res.Code >= 400 {
			res.Error = r.Status
		}

		res.Headers = r.Header

		if tgt.Next == nil {
			break
		} else {
			tgt = tgt.Next
		}
	}
	return &res
}
