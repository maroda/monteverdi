package plugin

/*
	CalcRate

	Returns a positive integer used to check for a Maxval

	~~~ Plugin Reference Implementation ~~~
*/

import (
	"time"
)

type CalcRatePlugin struct {
	PrevVal  map[string]int64
	PrevTime map[string]time.Time
}

// Transform is the main wrapper for the interface.
// Other calculation functions should be called from here.
func (p *CalcRatePlugin) Transform(metric string, current int64, historical []int64, timestamp time.Time) (int64, error) {
	// At least 1 historical measurement needed
	if len(historical) < 1 {
		return 0, nil
	}

	// Check the value in the plugin struct
	if prev, exists := p.PrevVal[metric]; exists {
		rate := CalcRate(current, prev, timestamp, p.PrevTime[metric])
		return rate, nil
	}

	// No rate, first time reading
	// If it's not even initialized, fix that too
	if p.PrevVal == nil {
		p.PrevVal = make(map[string]int64)
		p.PrevTime = make(map[string]time.Time)
	}
	p.PrevVal[metric] = current
	p.PrevTime[metric] = timestamp
	return 0, nil
}

// CalcRate is a generic rate calculator that
// receives two sequential events and their timestamps
// and returns a single integer as the rate (per second)
func CalcRate(curr, prev int64, currtime, prevtime time.Time) int64 {
	delta := curr - prev
	timeDelta := currtime.Sub(prevtime).Seconds()

	// Handle counter reset (to 0)
	if delta < 0 {
		delta = curr
	}

	return int64(float64(delta) / timeDelta)
}

func (p *CalcRatePlugin) HysteresisReq() int { return 1 }
func (p *CalcRatePlugin) Type() string       { return "calc_rate" }
