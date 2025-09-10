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

/*
// NewEndpoint returns a pointer to the Endpoint metadata and its data
// This function syncs endpoint with data using an index
func NewEndpoint(id, url string, m ...string) *Endpoint {
	collection := make(map[int]string)
	colldata := make(map[string]int64)

	// index for the metric collection
	index := len(m) - 1
	index = 0

	// Add values of entry parameters to the map as collection keys
	// Initialize the mdata[key] for this to zero
	for _, value := range m {
		collection[index] = value
		colldata[value] = 0
		index++
	}
	return &Endpoint{
		ID:     id,
		URL:    url,
		Metric: collection,
		Mdata:  colldata,
	}
}

*/

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) (*Endpoints, error) {
	var endpoints Endpoints

	// cf is a ConfigFile: ID, URL, MWithMax
	// on each member a new endpoint is created
	for _, c := range cf {
		// initialized for each Endpoint
		metric := make(map[int]string)

		// fmt.Printf("Processing config ID: %s\n", c.ID)
		// fmt.Printf("Config MWithMax: %+v\n", c.MWithMax)

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
			// Init all rune values - might not be necessary
			//for t := 0; t < tsdbWindow; t++ {
			//	metsdb[k].Runes[t] = rune(0)
			//}
			j++
		}

		// fmt.Printf("Final metric map: %+v\n", metric)

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
	// Lock endpoint for writing
	q.Network[i].MU.Lock()
	defer q.Network[i].MU.Unlock()

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

		slog.Debug("ACCENT FOUND",
			slog.Int64("Mdata", md),
			slog.Int64("Maxval", mx),
			slog.Int("Intensity", intensity),
			slog.Int64("Timestamp", a.Timestamp),
		)

		isAccent = true
	}

	// ALWAYS add to timeline, regardless of accent status
	// This will take the metric and fill
	// the rune at the current Counter location
	q.Network[i].AddSecondWithCheck(m, isAccent)
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

		// pollSource is a map of KV
		pollSource, err := MetricKV(delimiter, q.Network[ni].URL)
		if err != nil {
			slog.Error("Could not poll metric", slog.Any("Error", err))
			return err
		}

		// nv.Metric is the list of configured metrics we want to use
		for _, mv := range nv.Metric {
			// pollSource is the full list of metrics from above,
			for k := range pollSource {
				if k == mv {
					// we've found the key, now grab its metric from the poll
					// convert the metric to int64 on assignment
					metric, err = strconv.ParseInt(pollSource[mv], 10, 64)
					if err != nil {
						slog.Error("Could not convert metric to 64bits", slog.Any("Error", err))
						return err
					}

					// Populate the map in the struct
					q.Network[ni].Mdata[mv] = metric

					// Find any Accents at the same time
					accent := q.FindAccent(mv, ni)
					if accent == nil {
						slog.Debug("ACCENT EMPTY: NIL")
					}

				}
			}
		}
	}

	return nil
}
