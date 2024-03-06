package gogeta

import (
	"fmt"
	"math"
	"time"
)

type Pacer interface {
	// Pace returns the duration an Attacker should wait until
	// hitting the next Target, given an already elapsed duration and
	// completed hits. If the second return value is true, an attacker
	// should stop sending hits.
	Pace(elapsed time.Duration, hits uint64) (wait time.Duration, stop bool)

	// Rate returns a Pacer's instantaneous hit rate (per seconds)
	// at the given elapsed duration of an attack.
	Rate(elapsed time.Duration) float64
}

type ConstantPacer struct {
	Freq int
	Per  time.Duration
}

// String returns a pretty-printed description of the ConstantPacer's behaviour:
//
//	ConstantPacer{Freq: 1, Per: time.Second} => Constant{1 hits/1s}
func (c ConstantPacer) String() string {
	return fmt.Sprintf("Constant{%d hits/%s}", c.Freq, c.Per)
}

func (c ConstantPacer) Pace(elapsed time.Duration, hits uint64) (wait time.Duration, stop bool) {
	switch {
	case c.Per == 0 || c.Freq == 0:
		return 0, false // Zero value = infinite rate
	case c.Per < 0 || c.Freq < 0:
		return 0, true
	}

	expectedHits := uint64(c.Freq) * uint64(elapsed/c.Per)
	if hits < expectedHits {
		// Running behind, send next hit immediately.
		return 0, false
	}
	interval := uint64(c.Per.Nanoseconds() / int64(c.Freq))
	if math.MaxInt64/interval < hits {
		// We would overflow delta if we continued, so stop the attack.
		return 0, true
	}
	delta := time.Duration((hits + 1) * interval)
	// Zero or negative durations cause time.Sleep to return immediately.
	return delta - elapsed, false
}

func (c ConstantPacer) Rate(elapsed time.Duration) float64 {
	return c.hitsPerNs() * 1e9
}

func (c ConstantPacer) hitsPerNs() float64 {
	return float64(c.Freq) / float64(c.Per)
}

// Rate is a type alias for ConstantPacer for backwards-compatibility.
type Rate = ConstantPacer

// ConstantPacer satisfies the Pacer interface.
var _ Pacer = ConstantPacer{}
