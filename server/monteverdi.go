package monteverdi

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Mt "github.com/maroda/monteverdi/types"
)

// QNet represents the entire connected Network of Qualities
// This is where it is configured which sources are being used
// And where pointers to the data are held

type QNet struct {
	MU      sync.RWMutex
	Network Endpoints        // slice of *Endpoint
	Output  Mp.OutputAdapter // Output interface
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
	MU           sync.RWMutex
	ID           string                          // string describing the endpoint source
	URL          string                          // URL endpoint for the service
	Delim        string                          // delimiter for KV, empty for blobs
	Interval     time.Duration                   // interval to poll for metrics
	Metric       map[int]string                  // map of all metric keys to be retrieved
	Mdata        map[string]int64                // map of all metric data by metric key
	Maxval       map[string]int64                // map of metric data max val to find accents
	Transformers map[string]Mp.MetricTransformer // map of metric transformers in use
	Hysteresis   map[string]*CycBuffer           // map of buffers holding a history of metric data
	Accent       map[string]*Mt.Accent           // map of accents by metric key, timestamped
	Layer        map[string]*Mt.Timeseries       // map of rolling timeseries by metric key
	Sequence     map[string]*IctusSequence       // map of total timeseries for pattern matching
	Pulses       *TemporalGrouper                // accent groups arranged by pattern in time
}

type Endpoints []*Endpoint

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) *Endpoints {
	var endpoints Endpoints

	// TUI TSDB display (80 chars wide)
	tsdbWindow := FillEnvVarInt("MONTEVERDI_TUI_TSDB_VISUAL_WINDOW", 80)

	// Pulse lifecycle window (1 hour for full ring progression)
	pulseWindow := FillEnvVarInt("MONTEVERDI_PULSE_WINDOW_SECONDS", 3600)

	// cf is a ConfigFile (JSON) of Endpoints
	// in the format: ID, URL, MWithMax
	//
	// This initializes each Endpoint
	for _, c := range cf {
		// DEBUG ::: fmt.Printf("Processing config ID: %s\n", c.ID)
		// DEBUG ::: fmt.Printf("Config MWithMax: %+v\n", c.MWithMax)

		// All string Keys are /metric/
		metric := make(map[int]string)                        // The metric name
		mdata := make(map[string]int64)                       // Data
		maxval := make(map[string]int64)                      // Value triggers an accent
		hysteresis := make(map[string]*CycBuffer)             // Hysteresis buffer
		transformers := make(map[string]Mp.MetricTransformer) // Transformers in use
		accent := make(map[string]*Mt.Accent)                 // The accent metadata
		metsdb := make(map[string]*Mt.Timeseries)             // Timeseries tracking accents
		ictseq := make(map[string]*IctusSequence)             // Running change Sequence
		pulses := &TemporalGrouper{
			WindowSize: time.Duration(pulseWindow) * time.Second, // This is a display config
			Buffer:     make([]Mt.PulseEvent, 0),
			Groups:     make([]*Mt.PulseTree, 0),
		} // Group patterns in time

		// This locates the desired metrics from the on-disk config
		j := 0
		for k, mc := range c.Metrics {
			metric[j] = k               // assign the metric name from the config key
			maxval[k] = mc.Max          // assign the metric max value from the config value
			metsdb[k] = &Mt.Timeseries{ // create a new rolling tsdb for this metric
				Runes:   make([]rune, tsdbWindow),
				MaxSize: tsdbWindow,
				Current: 0,
			}
			if mc.Transformer != "" { // initialize transformer plugin if configured
				switch mc.Transformer {
				case "calc_rate":
					transformers[k] = &Mp.CalcRatePlugin{
						PrevVal:  make(map[string]int64),
						PrevTime: make(map[string]time.Time),
					}
				case "json_key":
					transformers[k] = &Mp.JSONKeyPlugin{MetricKey: k}
				}
			}

			j++
		}

		// Set polling interval, 15s is the default
		interval := time.Duration(c.Interval) * time.Second
		if interval == 0 {
			interval = 15 * time.Second
		}

		// Assign data we know, initialize data we don't
		NewEP := Endpoint{
			ID:           c.ID,
			URL:          c.URL,
			Delim:        c.Delim,
			Metric:       metric,
			Mdata:        mdata,
			Interval:     interval,
			Hysteresis:   hysteresis,
			Transformers: transformers,
			Maxval:       maxval,
			Accent:       accent,
			Layer:        metsdb,
			Sequence:     ictseq,
			Pulses:       pulses,
		}
		endpoints = append(endpoints, &NewEP)
	}
	return &endpoints
}

// AddSecondWithCheck tallies each second as a counter
// then adds a rune to the slice indexed by second
func (ep *Endpoint) AddSecondWithCheck(m string, isAccent bool) {
	// This is the index of the rune, and also the current second
	ep.Layer[m].Current = (ep.Layer[m].Current + 1) % ep.Layer[m].MaxSize

	// translate this val into a rune for display
	// ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRuneWithCheck(ep.Mdata[m], isAccent)
	ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRuneWithCheckMax(ep.Mdata[m], ep.Maxval[m], isAccent)
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
			Events:    make([]Mt.Ictus, 0),
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
	ictus := Mt.Ictus{
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
func (ep *Endpoint) GetPulseVizData(m string, fp *Mt.PulsePattern) []Mt.PulseVizPoint {
	ep.MU.RLock()
	defer ep.MU.RUnlock()

	if ep.Pulses == nil {
		// fmt.Printf("DEBUG: Pulses is nil for %s\n", m)
		return []Mt.PulseVizPoint{}
	}

	// Use a map to store the list
	// and deduplicate pulses where necessary,
	// giving display priority to the Trochee
	// (configured in ChooseBetterPoint)
	pointMap := make(map[int]Mt.PulseVizPoint)
	now := time.Now()

	// Track processed pulses to avoid duplicates
	seenPulses := make(map[string]bool)

	// Helper to create unique pulse key
	createPulseKey := func(pulse Mt.PulseEvent) string {
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
	points := make([]Mt.PulseVizPoint, 0, len(pointMap))
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

func (ep *Endpoint) PulseToPoints(pulse Mt.PulseEvent, now time.Time) []Mt.PulseVizPoint {
	var points []Mt.PulseVizPoint

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

		points = append(points, Mt.PulseVizPoint{
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

func (ep *Endpoint) CalcAccentStateForPos(pulse Mt.PulseEvent, pos, startPos, endPos int) bool {
	switch pulse.Pattern {
	case Mt.Iamb:
		// non-accent → accent: first half false, second half true
		midPoint := startPos + ((endPos - startPos) / 2)
		return pos >= midPoint
	case Mt.Trochee:
		// accent → non-accent: first half true, second half false
		midPoint := startPos + ((endPos - startPos) / 2)
		return pos < midPoint
	case Mt.Amphibrach:
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
func (q *QNet) FindAccent(m string, i int) *Mt.Accent {
	// Lock for the entire accent detection + timeline updates
	q.Network[i].MU.Lock()
	defer q.Network[i].MU.Unlock()

	// The metric data
	md := q.Network[i].Mdata[m]

	// The metric max
	mx := q.Network[i].Maxval[m]

	// init values
	intensity := 1
	a := &Mt.Accent{}
	isAccent := false

	// if the accent exists, fill in a bunch of metadata
	if md >= mx {
		q.Network[i].Accent = make(map[string]*Mt.Accent)
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

	// Trim sequence to prevent unbounded growth in seq.Events
	const maxRecognitionEvents = 10
	if seq != nil && len(seq.Events) > maxRecognitionEvents {
		seq.Events = seq.Events[len(seq.Events)-maxRecognitionEvents:]
		seq.LastProcessedEventCount = 0
	}

	// This isn't useful until there is at least one IctusPattern present
	// if seq != nil && len(seq.Events) >= 2 {
	if seq != nil && len(seq.Events) >= 2 && len(seq.Events) > seq.LastProcessedEventCount {
		// Only process new events, not the entire history
		newEventCount := len(seq.Events) - seq.LastProcessedEventCount
		startIdx := seq.LastProcessedEventCount
		if startIdx == 0 && len(seq.Events) >= 3 {
			// First run: process from the beginning but need at least 3 events
			startIdx = 0
		} else if newEventCount < 2 {
			// Need at least 2 new events to form patterns
			return
		}

		// Create sequence with only relevant events for pattern detection
		// Include overlap of 1 for pattern continuity
		overlapStart := Max(0, startIdx-1)
		relevantEvents := seq.Events[overlapStart:]

		// Convert to IctusSequence format first
		ictusSeq := &IctusSequence{
			Metric: m,
			Events: make([]Mt.Ictus, len(relevantEvents)),
		}

		for j, e := range relevantEvents {
			ictusSeq.Events[j] = Mt.Ictus{
				Timestamp: e.Timestamp,
				IsAccent:  e.IsAccent,
				Value:     e.Value,
				Duration:  e.Duration,
			}
		}

		// Detect new pulses and add to the temporal grouper
		// Tuning period ratios in /config/ is important for pulse detection
		config := NewPulseConfig(0.5, 0.5, 0.5, 0.5)
		pulses := ictusSeq.DetectPulsesWithConfig(*config)

		for _, pulse := range pulses {
			// Add the pulse itself
			q.Network[i].Pulses.AddPulse(pulse)

			slog.Debug("ADD PULSE", slog.Any("pattern", pulse.Pattern), slog.String("metric", m), slog.String("duration", pulse.Duration.String()))

			// If configured, use the Output Adapter Plugin
			// The output type depends on the value of MONTEVERDI_OUTPUT
			if q.Output != nil {
				if err := q.Output.WritePulse(&pulse); err != nil {
					slog.Error("Output adapter write failed",
						slog.String("metric", m),
						slog.String("error", err.Error()))
				}
			}
		}

		// Update processed count
		seq.LastProcessedEventCount = len(seq.Events)
	}
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PollEndpoint takes the Network index and fetches the metric
func (q *QNet) PollEndpoint(ni int) {
	var mname string
	var mdata, tdata int64
	var dataSink bool
	delimiter := q.Network[ni].Delim

	// When Delimiter is set to /""/ the entire fetch is the data
	if delimiter == "" {
		slog.Debug("Empty delimiter configured, using Transformer")
		slog.Debug("JSON poll started",
			slog.String("endpoint", q.Network[ni].ID),
			slog.Time("time", time.Now()))

		// To keep the atomicity of the metric name with its transformer key,
		// load the body early and operate on that with the transformer.
		code, metricsBlob, err := SingleFetch(q.Network[ni].URL)
		if err != nil {
			slog.Error("Problem fetching body",
				slog.String("url", q.Network[ni].URL),
				slog.Int("code", code),
				slog.Any("error", err))
		}
		slog.Debug("Fetch result",
			slog.Int("status_code", code),
			slog.Int("body_length", len(metricsBlob)),
			slog.String("body", string(metricsBlob)))

		// The metric key name in the configuration is the search term for the Transformer.
		// Step through each of them, using the metricsBlob body, and locate their values.
		for _, mname = range q.Network[ni].Metric {
			// Get the transformer
			mt := q.Network[ni].Transformers[mname]

			slog.Debug("Adding metric", slog.String("metric", mname), slog.Any("transformer", mt))

			// Currently only JSON has a plugin, but others could be supported
			switch mt.Type() {
			case "json_key":
				// Boolean used to register non-accents for display when set to false
				dataSink = true

				// Send the JSON as a string to match the interface args
				tdata, err = mt.Transform(string(metricsBlob), 0, []int64{0}, time.Now())
				if err != nil {
					slog.Error("Transformer error", slog.String("metric", mname), slog.String("error", err.Error()))
					// No accent to display because the metric is null
					dataSink = false
				}

				// Lock and record data
				q.Network[ni].MU.Lock()

				if dataSink {
					q.Network[ni].ValueToHysteresis(mname, tdata)
					q.Network[ni].Mdata[mname] = tdata
				}

				// Unlock and find the accent state
				q.Network[ni].MU.Unlock()
				q.FindAccent(mname, ni)
			}
		}
	}

	// Any other Delimiter is valid as a K/V pair
	// pollSource is a map of KV extracted from the remote side
	pollSource, err := MetricKV(delimiter, q.Network[ni].URL)
	if err != nil {
		slog.Error("Could not poll metric", slog.Any("Error", err))
	}

	// For each metric in the configuration:
	for _, mname = range q.Network[ni].Metric {
		// ... search through the polled data for its match
		for k, v := range pollSource {
			if k == mname {
				// We've found the key! now grab from the poll
				// make floats (e.g. exponential notation) become big integers
				if floatVal, err := strconv.ParseFloat(v, 64); err != nil {
					slog.Error("invalid syntax in metric", slog.Any("Error", err))
					continue
				} else {
					mdata = int64(floatVal) // Convert float to int64
				}

				// Lock endpoint for the entire op
				q.Network[ni].MU.Lock()

				// Record data to the hysteresis buffer
				// This comes before the Transformer so that it can be part of the calculation
				q.Network[ni].ValueToHysteresis(mname, mdata)

				// If a Transformer plugin is detected, use it
				transformers := q.Network[ni].Transformers

				// Reset dataSink for placing non-accents
				dataSink = true

				if transformers != nil {
					mt := transformers[mname] // e.g.: Mp.CalcRatePlugin
					if mt != nil {
						tdata, err = mt.Transform(mname, mdata, q.Network[ni].GetHysteresis(mname, mt.HysteresisReq()), time.Now())
						if err != nil {
							// Keep going, log the error, do not write any data
							slog.Error("Error transforming metric", slog.Any("Error", err))
							dataSink = false
						} else {
							// No check for dataSink here, we know it's true
							mdata = tdata
						}
					}
				}

				// Record data to Endpoint
				if dataSink {
					q.Network[ni].Mdata[mname] = mdata // Populate the map in the struct
				}

				// Unlock and find the accent state
				q.Network[ni].MU.Unlock()
				q.FindAccent(mname, ni)

				// Go to the top after we've processed this metric
				break
			}
		}
	}
}

// CycBuffer is a cyclic buffer for hysteresis,
// where MaxSize number of Mdata values are kept.
type CycBuffer struct {
	Values  []int64
	MaxSize int
	Index   int
}

// ValueToHysteresis records metric data to a cyclical buffer
// Currently configured with a static length of 20
// NB: Caller must hold ep.MU.Lock()
func (ep *Endpoint) ValueToHysteresis(metric string, value int64) {
	// Initialize the map if it doesn't exist
	if ep.Hysteresis == nil {
		ep.Hysteresis = make(map[string]*CycBuffer)
	}

	// Initialize the buffer if it doesn't exist, Index will be 0
	if ep.Hysteresis[metric] == nil {
		ep.Hysteresis[metric] = &CycBuffer{
			Values:  make([]int64, 20), // Initialize at 20 values
			MaxSize: 20,                // Limit on this buffer is 20
		}
	}

	// Get the buffer
	buffer := ep.Hysteresis[metric]

	// Assign the metric value to the buffer
	buffer.Values[buffer.Index] = value

	// Index always points to the next empty slot
	buffer.Index = (buffer.Index + 1) % buffer.MaxSize
}

// GetHysteresis retrieves a depth of metrics to use for calculations
// NB: Caller must hold ep.MU.Lock()
func (ep *Endpoint) GetHysteresis(metric string, depth int) []int64 {
	buffer := ep.Hysteresis[metric]
	if buffer == nil {
		// No buffer! So no depth to report.
		return []int64{}
	}

	// Clamp depth to the max buffer size
	if depth > buffer.MaxSize {
		depth = buffer.MaxSize
	}

	// Slice size equals the requested depth
	result := make([]int64, 0, depth)

	// Read backwards from current Index to get chronology
	for i := 0; i < depth; i++ {
		idx := (buffer.Index - 1 - i + buffer.MaxSize) % buffer.MaxSize
		result = append(result, buffer.Values[idx])
	}

	return result
}
