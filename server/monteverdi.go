package monteverdi

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// QNet represents the entire connected Network of Qualities
// This is where it is configured which sources are being used
// And where pointers to the data are held

type Monteverdi interface {
	FindAccent(string, int) *Accent
	Poll() (int64, error)
	PollMulti() error
}

type QNet struct {
	MU      sync.RWMutex
	Network Endpoints // slice of *Endpoint
}

// NewQNet creates a new Quality Network
// This contains a bunch of metadata
// The Endpoint object has pointers to its own data
// so this Endpoints slice contains everything
// and each time the QNet object is used
// it should have updated metrics via the Endpoints
//
// it's likely this metric updating will be goroutines
// for example step through the Endpoints slice
// and fire off a goroutine for each Endpoint
func NewQNet(ep Endpoints) *QNet {
	return &QNet{
		Network: ep,
	}
}

type EndpointOperate interface {
	AddSecond(string)
	AddSecondWithCheck(string, bool)
	ValToRuneWithCheck(int64, bool) rune
	ValToRune(int64) rune
	GetDisplay(string) []rune
}

// Endpoint is what the app uses to
// check the URL for whatever comes back
// and then use each metric listed in the map to grab data
//
//  1. app starts, reads config file. maybe this is TOML, or
//     another idea would be to have an API that takes a JSON config
//  2. this config is read into a slice of Endpoint entries
//  3. This slice - Endpoints - is fed into the machine
//  4. The QNet struct contains pointers to the data itself
//  5. The map of Accent is an on-only entry. If the raw Mdata
//     does not trigger an Accent, there is no entry for that timestamp.
//     However, the Accent is always located by the Metric key itself.
//     The display can be configured to show a certain number of Accents.
type Endpoint struct {
	MU       sync.RWMutex
	ID       string                    // string describing the endpoint source
	URL      string                    // URL endpoint for the service
	Delim    string                    // delimiter for KV
	Metric   map[int]string            // map of all metric keys to be retrieved
	Mdata    map[string]int64          // map of all metric data by metric key
	Maxval   map[string]int64          // map of metric data max val to find accents
	Accent   map[string]*Accent        // map of accents by metric key, timestamped
	Layer    map[string]*Timeseries    // map of rolling timeseries by metric key
	Sequence map[string]*IctusSequence // map of total timeseries for pattern matching
	Pulses   *TemporalGrouper          // accent groups arranged by pattern in time
}

type Endpoints []*Endpoint

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) (*Endpoints, error) {
	var endpoints Endpoints
	tsdbWindow := FillEnvVarInt("MONTEVERDI_TUI_TSDB_VISUAL_WINDOW", 80)

	// cf is a ConfigFile (JSON) of Endpoints
	// in the format: ID, URL, MWithMax
	//
	// This initializes each Endpoint
	for _, c := range cf {
		// DEBUG ::: fmt.Printf("Processing config ID: %s\n", c.ID)
		// DEBUG ::: fmt.Printf("Config MWithMax: %+v\n", c.MWithMax)

		// All string Keys are /metric/
		metric := make(map[int]string)            // The metric name
		mdata := make(map[string]int64)           // Data
		maxval := make(map[string]int64)          // Value triggers an accent
		accent := make(map[string]*Accent)        // The accent metadata
		metsdb := make(map[string]*Timeseries)    // Timeseries tracking accents
		ictseq := make(map[string]*IctusSequence) // Running change Sequence
		pulses := &TemporalGrouper{
			WindowSize: time.Duration(tsdbWindow) * time.Second,
			Buffer:     make([]PulseEvent, 0),
			Groups:     make([]*PulseTree, 0),
		} // Group patterns in time

		// This locates the desired metrics from the on-disk config
		j := 0
		for k, v := range c.MWithMax {
			metric[j] = k            // assign the metric name from the config key
			maxval[k] = int64(v)     // assign the metric max value from the config value
			metsdb[k] = &Timeseries{ // create a new rolling tsdb for this metric
				Runes:   make([]rune, tsdbWindow),
				MaxSize: tsdbWindow,
				Current: 0,
			}
			j++
		}

		// DEBUG ::: fmt.Printf("Final metric map: %+v\n", metric)

		// Assign data we know, initialize data we don't
		NewEP := Endpoint{
			ID:       c.ID,
			URL:      c.URL,
			Delim:    c.Delim,
			Metric:   metric,
			Mdata:    mdata,
			Maxval:   maxval,
			Accent:   accent,
			Layer:    metsdb,
			Sequence: ictseq,
			Pulses:   pulses,
		}
		endpoints = append(endpoints, &NewEP)
	}
	return &endpoints, nil
}

// AddSecondWithCheck tallies each second as a counter
// then adds a rune to the slice indexed by second
func (ep *Endpoint) AddSecondWithCheck(m string, isAccent bool) {
	// This is the index of the rune, and also the current second
	ep.Layer[m].Current = (ep.Layer[m].Current + 1) % ep.Layer[m].MaxSize

	// translate this val into a rune for display
	// ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRuneWithCheck(ep.Mdata[m], isAccent)
	ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRuneWithCheckMax(ep.Mdata[m], ep.Maxval[m], isAccent)

	// TODO: Can the value range be derived from or relative to ep.maxval?
}

func (ep *Endpoint) ValToRuneWithCheckMax(val, max int64, isAccent bool) rune {
	if !isAccent {
		return ' '
	}

	r := max / 8

	switch {
	case val < max+r:
		return '▁'
	case val < max+(2*r):
		return '▂'
	case val < max+(3*r):
		return '▃'
	case val < max+(4*r):
		return '▄'
	case val < max+(5*r):
		return '▅'
	case val < max+(6*r):
		return '▆'
	case val < max+(7*r):
		return '▇'
	default:
		return '█'
	}
}

// GetDisplay provides the string of runes for drawing using the metric name
func (ep *Endpoint) GetDisplay(m string) []rune {
	display := make([]rune, ep.Layer[m].MaxSize)
	for i := 0; i < ep.Layer[m].MaxSize; i++ {
		// Start from oldest and go to newest (left to right)
		// subtract 1 instead to go right->left
		idx := (ep.Layer[m].Current + 1 + i) % ep.Layer[m].MaxSize
		display[i] = ep.Layer[m].Runes[idx]
	}
	return display
}

// RecordIctus takes the metric, accent state, and metric data (pointer)
// and records either an ongoing duration if an existing state is continuing,
// or it creates a new start for a new state.
func (ep *Endpoint) RecordIctus(m string, isAccent bool, d int64) {
	now := time.Now()

	// Initialize the sequence map if it doesn't exist for this metric
	if ep.Sequence[m] == nil {
		ep.Sequence[m] = &IctusSequence{
			Metric:    m,
			Events:    make([]Ictus, 0),
			StartTime: now,
		}
	}

	seq := ep.Sequence[m]

	// Are we changing state from or to accent?
	if len(seq.Events) > 0 {
		previousIctus := &seq.Events[len(seq.Events)-1]
		if previousIctus.IsAccent == isAccent {
			// It's the same so we update duration of the previous ictus
			previousIctus.Duration = now.Sub(previousIctus.Timestamp)
			slog.Debug("Ictus Update", slog.String("metric", m), slog.String("duration", previousIctus.Duration.String()))
			return
		}
	}

	// State is changing! Record the new Ictus
	ictus := Ictus{
		Timestamp: now,
		IsAccent:  isAccent,
		Value:     d,
		Duration:  0,
	}

	seq.Events = append(seq.Events, ictus)
	seq.EndTime = now
	slog.Debug("NEW Accent Ictus", slog.String("metric", m), slog.Int64("value", ictus.Value))
}

// GetPulseVizData takes a metric name and returns its viz point data
// Second argument is used to filter the results on a specific PulsePattern
func (ep *Endpoint) GetPulseVizData(m string, fp *PulsePattern) []PulseVizPoint {
	ep.MU.RLock()
	defer ep.MU.RUnlock()

	if ep.Pulses == nil {
		// fmt.Printf("DEBUG: Pulses is nil for %s\n", m)
		return []PulseVizPoint{}
	}

	// Use a map to store the list
	// and deduplicate pulses where necessary,
	// giving display priority to the Trochee
	// (configured in ChooseBetterPoint)
	pointMap := make(map[int]PulseVizPoint)
	now := time.Now()

	// Track processed pulses to avoid duplicates
	seenPulses := make(map[string]bool)

	// Helper to create unique pulse key
	createPulseKey := func(pulse PulseEvent) string {
		return fmt.Sprintf("%v_%d_%v", pulse.StartTime.UnixNano(), pulse.Pattern, pulse.Duration.Nanoseconds())
	}

	// Process pulses held in the buffer
	for _, pulse := range ep.Pulses.Buffer {
		// Apply pattern filter
		if fp != nil && pulse.Pattern != *fp {
			continue
		}

		if len(pulse.Metric) == 0 || contains(pulse.Metric, m) {
			key := createPulseKey(pulse)
			if !seenPulses[key] {
				seenPulses[key] = true
				pulsePoints := ep.PulseToPoints(pulse, now)
				for _, point := range pulsePoints {
					pointMap[point.Position] = point
				}
			}
		}
	}

	// Process pulses from completed groups
	tsdbWindow := FillEnvVarInt("MONTEVERDI_TUI_TSDB_VISUAL_WINDOW", 80)
	limiter := now.Add(-time.Duration(tsdbWindow) * time.Second)
	for _, group := range ep.Pulses.Groups {
		if group.StartTime.After(limiter) && len(group.OGEvents) > 0 {
			for _, pulse := range group.OGEvents {
				// Apply pattern filter
				if fp != nil && pulse.Pattern != *fp {
					continue
				}

				if len(pulse.Metric) == 0 || contains(pulse.Metric, m) {
					key := createPulseKey(pulse)
					if !seenPulses[key] {
						seenPulses[key] = true
						pulsePoints := ep.PulseToPoints(pulse, now)
						for _, point := range pulsePoints {
							pointMap[point.Position] = point
						}
					}
				}
			}
		}
	}

	// Convert map back to slice
	points := make([]PulseVizPoint, 0, len(pointMap))
	for _, point := range pointMap {
		points = append(points, point)
	}

	return points
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func (ep *Endpoint) PulseToPoints(pulse PulseEvent, now time.Time) []PulseVizPoint {
	var points []PulseVizPoint

	// Calculate timeline position
	tsdbWindow := FillEnvVarInt("MONTEVERDI_TUI_TSDB_VISUAL_WINDOW", 80)
	secAgo := int(now.Sub(pulse.StartTime).Seconds())
	startPos := (tsdbWindow - 1) - secAgo
	durWidth := int(pulse.Duration.Seconds())

	// Track if pulse extends beyond visible range
	extends := startPos < 0

	// Clamp to visible range
	if startPos < 0 {
		startPos = 0
	}
	endPos := startPos + durWidth
	if endPos >= tsdbWindow {
		endPos = tsdbWindow - 1
	}

	// Generate points for each timeline position this pulse occupies
	for pos := startPos; pos <= endPos; pos++ {
		isAccent := ep.CalcAccentStateForPos(pulse, pos, startPos, endPos)

		points = append(points, PulseVizPoint{
			Position:  pos,
			Pattern:   pulse.Pattern,
			IsAccent:  isAccent,
			Duration:  pulse.Duration,
			StartTime: pulse.StartTime,
			Extends:   extends && pos == 0, // Only mark leftmost visible
		})
	}

	return points
}

func (ep *Endpoint) CalcAccentStateForPos(pulse PulseEvent, pos, startPos, endPos int) bool {
	switch pulse.Pattern {
	case Iamb:
		// non-accent → accent: first half false, second half true
		midPoint := startPos + ((endPos - startPos) / 2)
		return pos >= midPoint
	case Trochee:
		// accent → non-accent: first half true, second half false
		midPoint := startPos + ((endPos - startPos) / 2)
		return pos < midPoint
	case Amphibrach:
		// non-accent → accent → non-accent: first third false, middle third true, last third false
		firstThird := startPos + ((endPos - startPos) / 3)
		secondThird := startPos + (2 * (endPos - startPos) / 3)
		return pos >= firstThird && pos < secondThird
	}
	return false
}

// FindAccent is configured with the parameters applied to a single metric
// for now, intensity is 1 for everything later on, intensity is used to
// 'weight' metrics i.e. give them greater harmonic meaning.
//
// i == Network index
// p == Metric name
func (q *QNet) FindAccent(m string, i int) *Accent {
	// The metric data
	md := q.Network[i].Mdata[m]

	// The metric max
	mx := q.Network[i].Maxval[m]

	// init values
	intensity := 1
	a := &Accent{}
	isAccent := false

	// if the accent exists, fill in a bunch of metadata
	if md >= mx {
		q.Network[i].Accent = make(map[string]*Accent)
		q.Network[i].Accent[m] = NewAccent(intensity, m)
		a = q.Network[i].Accent[m]
		isAccent = true
	}

	// ALWAYS add to timeline, regardless of accent status
	// This will take the metric and fill
	// the rune at the current Counter location
	q.Network[i].AddSecondWithCheck(m, isAccent)

	// Record an Ictus or update the previously
	// recorded Ictus duration in the sequence
	q.Network[i].RecordIctus(m, isAccent, md)

	// Run pulse detection on the updated sequence
	q.PulseDetect(m, i)

	return a
}

func (q *QNet) PulseDetect(m string, i int) {
	// retrieve an updated sequence
	seq := q.Network[i].Sequence[m]

	// This isn't useful until there is at least one IctusPattern present
	if seq != nil && len(seq.Events) >= 2 { // characters in one IctusPattern
		// Convert to IctusSequence format first
		ictusSeq := &IctusSequence{
			Metric: m,
			Events: make([]Ictus, len(seq.Events)),
		}

		for j, e := range seq.Events {
			ictusSeq.Events[j] = Ictus{
				Timestamp: e.Timestamp,
				IsAccent:  e.IsAccent,
				Value:     e.Value,
				Duration:  e.Duration,
			}
		}

		// Tuning period ratios is important for pulse detection
		config := NewPulseConfig(0.5, 0.5, 0.5, 0.5)

		// Detect new pulses and add to the temporal grouper
		pulses := ictusSeq.DetectPulsesWithConfig(*config)
		for _, pulse := range pulses {
			// Add the pulse itself
			q.Network[i].Pulses.AddPulse(pulse)

			slog.Debug("ADD PULSE", slog.Any("pattern", pulse.Pattern), slog.String("metric", m), slog.String("duration", pulse.Duration.String()))
		}
	}
}

// PollMulti reads all configured metrics from QNet and retrieves them.
func (q *QNet) PollMulti() error {
	var metric int64
	metric = 0

	// Default delimiter is /=/
	var delimiter string
	delimiter = "="

	// Step through all Networks in QNet
	for ni, nv := range q.Network {
		// get custom delimiter
		delimiter = q.Network[ni].Delim

		// pollSource is a map of KV extracted from the remote side
		// map[METRIC_NAME]METRIC_VAL_STRING
		pollSource, err := MetricKV(delimiter, q.Network[ni].URL)
		if err != nil {
			slog.Error("Could not poll metric", slog.Any("Error", err))
			return err
		}

		// nv.Metric is the list of configured metrics we want to use
		for _, mv := range nv.Metric {
			// pollSource is the full list of metrics from above
			for k, v := range pollSource {
				if k == mv {
					// we've found the key, now grab its metric from the poll
					// convert the metric to int64 on assignment
					if floatVal, err := strconv.ParseFloat(v, 64); err != nil {
						slog.Error("invalid syntax in metric", slog.Any("Error", err))
						return err
					} else {
						metric = int64(floatVal) // Convert float to int64
					}

					// Lock endpoint for the entire op
					q.Network[ni].MU.Lock()

					// Populate the map in the struct
					q.Network[ni].Mdata[mv] = metric

					// Find any Accents at the same time
					q.FindAccent(mv, ni)

					q.Network[ni].MU.Unlock()
				}
			}
		}
	}

	return nil
}
