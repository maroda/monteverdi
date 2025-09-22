package monteverdi

import (
	"math"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	Ms "github.com/maroda/monteverdi/server"
)

type PulseDataD3 struct {
	Ring      int     `json:"ring"`      // 0=60sec, 1=10min, 2=1hr
	Angle     float64 `json:"angle"`     // 0-360 degrees
	Type      string  `json:"type"`      // "iamb" or "trochee"
	Intensity float64 `json:"intensity"` // 0.0-1.0
	Speed     float64 `json:"speed"`     // degrees per frame
	Metric    string  `json:"metric"`    // Which system metric
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (v *View) websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send pulse data periodically
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pulseData := v.GetPulseDataD3()
			if err := conn.WriteJSON(pulseData); err != nil {
				return // Connection closed
			}
		}
	}
}

func (v *View) GetPulseDataD3() []PulseDataD3 {
	var pulses []PulseDataD3

	// Lock the QNet
	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	for _, endpoint := range v.QNet.Network {
		// Lock the Endpoint
		endpoint.MU.RLock()

		// Process endpoint data
		for metric, seq := range endpoint.Sequence {
			if seq != nil && len(seq.Events) > 0 {
				// Convert IctusSequence to D3 format
				recent := seq.DetectPulses()
				for _, pulse := range recent {
					d3pulse := PulseDataD3{
						Ring:      calcRing(pulse.StartTime),  // Based on age
						Angle:     calcAngle(pulse.StartTime), // Time-based position
						Type:      pulsePatternToString(pulse.Pattern),
						Intensity: calcIntensity(pulse.StartTime, endpoint),
						Speed:     v.calcSpeedForPulse(pulse),
						Metric:    metric,
					}
					pulses = append(pulses, d3pulse)
				}
			}
		}

		endpoint.MU.RUnlock()
	}
	return pulses
}

func pulsePatternToString(pattern Ms.PulsePattern) string {
	switch pattern {
	case Ms.Iamb:
		return "iamb"
	case Ms.Trochee:
		return "trochee"
	default:
		return "unknown"
	}
}

func calcAngle(ps time.Time) float64 {
	now := time.Now()
	age := now.Sub(ps)

	// Map age to angle, based on which ring it belongs
	ring := calcRing(ps)

	var windowDur time.Duration
	var angleInWindow float64

	switch ring {
	case 0: // Inner ring: 60s
		windowDur = 60 * time.Second
		// Recent pulses appear at "12 o'clock" (270°), older ones rotate clockwise
		angleInWindow = age.Seconds() / windowDur.Seconds()
	case 1: // Middle ring: 10m
		windowDur = 10 * time.Minute
		angleInWindow = age.Seconds() / windowDur.Seconds()
	case 2: // Outer ring: 1h
		windowDur = 1 * time.Hour
		angleInWindow = age.Seconds() / windowDur.Seconds()
	default:
		return 0
	}

	// Convert to degrees (0-360)
	// Start at 270° (12 o'clock) and rotate clockwise as age increases
	angle := 270.0 + (angleInWindow * 360.0)

	// Normalize to 0-360 range
	return math.Mod(angle, 360.0)
}

// ps = pulse start
// NB: endpoint.MU is already RLocked by the caller (GetPulseDataD3)
func calcIntensity(ps time.Time, ep *Ms.Endpoint) float64 {
	for metric, accent := range ep.Accent {
		if accent != nil {
			baseIntensity := float64(accent.Intensity) / 10.0 // Normalize?

			if mdata, exists := ep.Mdata[metric]; exists {
				if maxval, exists := ep.Maxval[metric]; exists && maxval > 0 {
					valueRatio := float64(mdata) / float64(maxval)
					intensity := baseIntensity * valueRatio

					// Clamp to 0.2-1.0 range
					return math.Max(math.Min(intensity, 1.0), 0.2)
				}
			}
		}
	}

	// Fallback intensity
	return 0.5
}

func calcRing(ps time.Time) int {
	now := time.Now()
	age := now.Sub(ps)

	switch {
	case age <= 60*time.Second:
		return 0 // Inner ring - last 60s
	case age <= 10*time.Minute:
		return 1 // Middle ring - last 10m
	case age <= 1*time.Hour:
		return 2 // Outer ring - last hour
	default:
		// Pulse is too old, do not display
		return -1
	}
}

type SpeedConfig struct {
	InnerBase  float64 // Ring 0 base speed
	MiddleBase float64 // Ring 1 base speed
	OuterBase  float64 // Ring 2 base speed
	GlobalBase float64 // Configurable global multiplier
}

// Base speeds for each ring (degrees per 50ms update)
// Inner ring completes full rotation in ~18 seconds (360° / 2°/frame * 50ms)
// Middle ring completes full rotation in ~36 seconds
// Outer ring completes full rotation in ~72 seconds
func calcSpeed(ps time.Time, config SpeedConfig) float64 {
	ring := calcRing(ps)

	var baseSpeed float64
	switch ring {
	case 0:
		baseSpeed = config.InnerBase
	case 1:
		baseSpeed = config.MiddleBase
	case 2:
		baseSpeed = config.OuterBase
	default:
		return 0.0
	}

	// Apply global multiplier last
	return baseSpeed * config.GlobalBase
}

func (v *View) calcSpeedForPulse(pe Ms.PulseEvent) float64 {
	// Default configuration - completely configurable
	config := SpeedConfig{
		InnerBase:  2.0,
		MiddleBase: 1.0,
		OuterBase:  0.5,
		GlobalBase: 1.0,
	}

	return calcSpeed(pe.StartTime, config)
}

// Mux handles both Prometheus metrics and WebSocket data delivery
func (v *View) setupMux() *mux.Router {
	r := mux.NewRouter()

	r.Handle("/metrics", v.stats.Handler())
	r.HandleFunc("/ws", v.websocketHandler)

	// Static files for D3 frontend
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web/")))

	api := r.PathPrefix("/api").Subrouter()
	api.Use(v.statsMiddleware)

	return r
}
