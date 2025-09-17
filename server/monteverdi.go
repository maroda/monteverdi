package monteverdi

import (
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
	Groups   *TemporalGrouper          // accent groups arranged by pattern in time
}

type Endpoints []*Endpoint

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) (*Endpoints, error) {
	var endpoints Endpoints

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
		groups := &TemporalGrouper{}              // Group patterns in time
		tsdbWindow := 60

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
			Groups:   groups,
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

// FindAccent is configured with the parameters applied to a single metric
// for now, intensity is 1 for everything later on, intensity is used to
// 'weight' metrics i.e. give them greater harmonic meaning.
// Currently destination layer is not used
//
// i == Network index
// p == Metric name
func (q *QNet) FindAccent(m string, i int) *Accent {
	// DEBUG ::: fmt.Printf("FindAccent: starting for metric %s, endpoint %d\n", m, i)

	// Lock endpoint for writing
	q.Network[i].MU.Lock()
	defer q.Network[i].MU.Unlock()

	// DEBUG ::: fmt.Printf("FindAccent: acquired lock\n")

	// The metric data
	md := q.Network[i].Mdata[m]
	// DEBUG ::: fmt.Printf("FindAccent: md = %d\n", md)

	// The metric max
	mx := q.Network[i].Maxval[m]
	// DEBUG ::: fmt.Printf("FindAccent: mx = %d\n", mx)

	// init values
	intensity := 1
	a := &Accent{}
	isAccent := false

	// if the accent exists, fill in a bunch of metadata
	if md >= mx {
		// DEBUG ::: fmt.Printf("FindAccent: creating accent\n")
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

		// Detect new pulses and add to the temporal grouper
		pulses := ictusSeq.DetectPulses()
		for _, pulse := range pulses {
			q.Network[i].Groups.AddPulse(pulse)
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

			/* DEBUG
			for k, v := range pollSource {
				fmt.Printf("pollSource key: '%s', value: '%s'\n", k, v)
			}
			for _, mv := range nv.Metric {
				fmt.Printf("Looking for metric: '%s'\n", mv)
			}
			*/

			for k, v := range pollSource {
				if k == mv {
					// we've found the key, now grab its metric from the poll
					// convert the metric to int64 on assignment

					// Add this debug output in your PollMulti method temporarily
					// DEBUG ::: fmt.Printf("pollSource contents: %+v\n", pollSource)
					// DEBUG ::: fmt.Printf("Looking for metric: %s\n", mv)

					if floatVal, err := strconv.ParseFloat(v, 64); err != nil {
						slog.Error("invalid syntax in metric", slog.Any("Error", err))
						return err
					} else {
						metric = int64(floatVal) // Convert float to int64
					}

					/*
						metric, err = strconv.ParseInt(v, 10, 64)
						if err != nil {
							slog.Error("invalid syntax in metric", slog.Any("Error", err))
							return err
						}
					*/

					// Populate the map in the struct
					q.Network[ni].Mdata[mv] = metric

					// DEBUG ::: fmt.Printf("About to call FindAccent for metric: %s\n", mv)

					// Find any Accents at the same time
					q.FindAccent(mv, ni)
					// DEBUG
					// accent := q.FindAccent(mv, ni)
					//if accent == nil {
					//	slog.Debug("ACCENT EMPTY: NIL")
					//}

					// DEBUG ::: fmt.Printf("Successfully processed metric: %s\n", mv)
				}
			}
		}
	}

	return nil
}
