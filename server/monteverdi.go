package monteverdi

import (
	"log/slog"
	"strconv"
	"sync"
)

// QNet represents the entire connected Network of Qualities
// This is where it is configured which sources are being used
// And where pointers to the data are held

type Monteverdi interface {
	FindAccent() *Accent
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
	MU     sync.RWMutex
	ID     string                 // string describing the endpoint source
	URL    string                 // URL endpoint for the service
	Delim  string                 // delimiter for KV
	Metric map[int]string         // map of all metric keys to be retrieved
	Mdata  map[string]int64       // map of all metric data by metric key
	Maxval map[string]int64       // map of metric data max val to find accents
	Accent map[string]*Accent     // map of accents by metric key, timestamped
	Layer  map[string]*Timeseries // map of rolling timeseries by metric key
}

type Endpoints []*Endpoint

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) (*Endpoints, error) {
	var endpoints Endpoints

	// cf is a ConfigFile: ID, URL, MWithMax
	// on each member a new endpoint is created
	for _, c := range cf {
		// initialized for each Endpoint
		metric := make(map[int]string)

		// DEBUG ::: fmt.Printf("Processing config ID: %s\n", c.ID)
		// DEBUG ::: fmt.Printf("Config MWithMax: %+v\n", c.MWithMax)

		mdata := make(map[string]int64)
		maxval := make(map[string]int64)
		accent := make(map[string]*Accent)
		metsdb := make(map[string]*Timeseries)
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
			ID:     c.ID,
			URL:    c.URL,
			Delim:  c.Delim,
			Metric: metric,
			Mdata:  mdata,
			Maxval: maxval,
			Accent: accent,
			Layer:  metsdb,
		}
		endpoints = append(endpoints, &NewEP)
	}
	return &endpoints, nil
}

// AddSecond tallies each second as a counter
// then adds a rune to the slice indexed by second
// this needs to take the metric name
// TODO: Clean this up, WithCheck should be the default
func (ep *Endpoint) AddSecond(m string) {
	// This is the index of the rune, and also the current second
	ep.Layer[m].Current = (ep.Layer[m].Current + 1) % ep.Layer[m].MaxSize

	// translate this val into a rune for display
	ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRune(ep.Mdata[m])
}

func (ep *Endpoint) AddSecondWithCheck(m string, isAccent bool) {
	// This is the index of the rune, and also the current second
	ep.Layer[m].Current = (ep.Layer[m].Current + 1) % ep.Layer[m].MaxSize

	// translate this val into a rune for display
	ep.Layer[m].Runes[ep.Layer[m].Current] = ep.ValToRuneWithCheck(ep.Mdata[m], isAccent)
}

func (ep *Endpoint) ValToRuneWithCheck(val int64, isAccent bool) rune {
	if !isAccent {
		return ' '
	}

	switch {
	case val < 10:
		return '▁'
	case val < 20:
		return '▂'
	case val < 30:
		return '▃'
	case val < 50:
		return '▄'
	case val < 80:
		return '▅'
	case val < 130:
		return '▆'
	case val < 210:
		return '▇'
	default:
		return '█'
	}
}

func (ep *Endpoint) ValToRune(val int64) rune {
	switch {
	case val < 10:
		return '▁'
	case val < 20:
		return '▂'
	case val < 30:
		return '▃'
	case val < 50:
		return '▄'
	case val < 80:
		return '▅'
	case val < 130:
		return '▆'
	case val < 210:
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

	// DEBUG ::: fmt.Printf("FindAccent: about to call AddSecondWithCheck\n")

	// ALWAYS add to timeline, regardless of accent status
	// This will take the metric and fill
	// the rune at the current Counter location
	q.Network[i].AddSecondWithCheck(m, isAccent)

	// DEBUG ::: fmt.Printf("FindAccent: AddSecondWithCheck completed\n")

	return a
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

					metric, err = strconv.ParseInt(v, 10, 64)
					if err != nil {
						slog.Error("invalid syntax in metric", slog.Any("Error", err))
						return err
					}

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
