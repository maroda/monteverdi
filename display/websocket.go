package monteverdi

import (
	"log/slog"
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
	Dimension int     `json:"dimension"` // Dimension for viz placement
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
	// Make sure we're not nil
	if v.QNet == nil || v.QNet.Network == nil {
		return []PulseDataD3{}
	}

	// Lock the QNet
	v.QNet.MU.RLock()
	defer v.QNet.MU.RUnlock()

	var pulses []PulseDataD3

	for _, endpoint := range v.QNet.Network {
		// Check for nil endpoints first
		if endpoint == nil {
			continue
		}

		// Lock the Endpoint
		endpoint.MU.RLock()

		// Process endpoint data
		for metric, seq := range endpoint.Sequence {
			if seq != nil && len(seq.Events) > 0 {
				// Convert IctusSequence to D3 format
				recent := seq.DetectPulses()
				for _, pulse := range recent {
					d3pulse := PulseDataD3{
						Ring:      CalcRing(pulse.StartTime),  // Based on age
						Angle:     CalcAngle(pulse.StartTime), // Time-based position
						Type:      PulsePatternToString(pulse.Pattern),
						Intensity: CalcIntensity(endpoint),
						Speed:     v.CalcSpeedForPulse(pulse),
						Metric:    metric,
						Dimension: pulse.Dimension,
					}
					pulses = append(pulses, d3pulse)
				}
			}
		}

		// Process D2 pulses from TemporalGrouper
		if endpoint.Pulses != nil {
			for _, pulse := range endpoint.Pulses.Buffer {
				if pulse.Dimension == 2 {
					for _, metric := range pulse.Metric {
						d3pulse := PulseDataD3{
							Ring:      CalcRing(pulse.StartTime),
							Angle:     CalcAngle(pulse.StartTime),
							Type:      PulsePatternToString(pulse.Pattern),
							Intensity: CalcIntensity(endpoint),
							Speed:     v.CalcSpeedForPulse(pulse),
							Metric:    metric,
							Dimension: pulse.Dimension,
						}
						pulses = append(pulses, d3pulse)
					}
				}
			}
		}

		endpoint.MU.RUnlock()
	}
	return pulses
}

func PulsePatternToString(pattern Ms.PulsePattern) string {
	switch pattern {
	case Ms.Iamb:
		return "iamb"
	case Ms.Trochee:
		return "trochee"
	case Ms.Amphibrach:
		return "amphibrach"
	case Ms.Anapest:
		return "anapest"
	case Ms.Dactyl:
		return "dactyl"
	default:
		return "unknown"
	}
}

func CalcAngle(ps time.Time) float64 {
	now := time.Now()
	age := now.Sub(ps)
	ring := CalcRing(ps)

	var angleInWindow float64

	switch ring {
	case 0:
		// Inner ring: 60s - full circle
		angleInWindow = age.Seconds() / 60.0
	case 1:
		// Middle ring: 10m - full circle, but start from when it entered this ring
		ageInRing := age.Minutes() - 1.0 // Subtract the 1 minute it spent in inner ring
		angleInWindow = ageInRing / 9.0  // 9 minutes remaining in middle ring
	case 2:
		// Outer ring: 1h - full circle, starting from when it entered this ring
		ageInRing := age.Minutes() - 10.0        // Subtract 10 minutes from previous rings
		angleInWindow = (ageInRing / 60.0) / 1.0 // Convert to hours for the 1-hour rotation
	default:
		return 0
	}

	// Start at 12 o'clock (270°) and rotate clockwise
	angle := 270.0 - (angleInWindow * 360.0)

	// Normalize to 0-360 range
	return math.Mod(angle+360.0, 360.0)
}

// CalcIntensity returns an intensity float
// ps = pulse start
// NB: endpoint.MU is already RLocked by the caller (GetPulseDataD3)
func CalcIntensity(ep *Ms.Endpoint) float64 {
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

func CalcRing(ps time.Time) int {
	now := time.Now()
	age := now.Sub(ps)

	if age.Seconds() > 59 && age.Seconds() < 61 {
		slog.Debug("RING_BOUNDARY_CHECK",
			slog.Float64("age_seconds", age.Seconds()),
			slog.Bool("exactly_60", age == 60*time.Second),
			slog.Bool("less_equal_60", age <= 60*time.Second))
	}

	switch {
	case age <= 60*time.Second:
		return 0 // Inner ring - last 60s
	case age <= 10*time.Minute:
		return 1 // Middle ring - last 10m
	case age <= 60*time.Minute:
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

// CalcSpeed has the following Base speeds for each ring (degrees per 50ms update)
// Inner ring completes full rotation in ~18 seconds (360° / 2°/frame * 50ms)
// Middle ring completes full rotation in ~36 seconds
// Outer ring completes full rotation in ~72 seconds
func CalcSpeed(ps time.Time, config SpeedConfig) float64 {
	ring := CalcRing(ps)

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

func (v *View) CalcSpeedForPulse(pe Ms.PulseEvent) float64 {
	// Default configuration - completely configurable
	config := SpeedConfig{
		InnerBase:  2.0,
		MiddleBase: 1.0,
		OuterBase:  0.5,
		GlobalBase: 1.0,
	}

	return CalcSpeed(pe.StartTime, config)
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
