package monteverdi

import (
	"log/slog"
	"strconv"
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
	ID     string             // string describing the endpoint source
	URL    string             // URL endpoint for the service
	Metric map[int]string     // map of all metric keys to be retrieved
	Mdata  map[string]int64   // map of all metric data by metric key
	Maxval map[string]int64   // map of metric data max val to find accents
	Accent map[string]*Accent // map of accents by metric key, timestamped
}

type Endpoints []Endpoint

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

// NewEndpointsFromConfig returns the slice of Endpoint containing all config stanzas
func NewEndpointsFromConfig(cf []ConfigFile) (*Endpoints, error) {
	var endpoints Endpoints
	metric := make(map[int]string)
	maxval := make(map[string]int64)

	// cf is a ConfigFile: ID, URL, MWithMax
	// on each member a new endpoint is created
	// we don't need the index yet? it is the count of all config items
	for _, c := range cf {
		j := 0
		for k, v := range c.MWithMax {
			metric[j] = k
			maxval[k] = int64(v)
			j++
		}
		// Assign data we know, initialize data we don't
		NewEP := &Endpoint{
			ID:     c.ID,
			URL:    c.URL,
			Metric: metric,
			Mdata:  map[string]int64{},
			Maxval: maxval,
			Accent: map[string]*Accent{},
		}
		endpoints = append(endpoints, *NewEP)
	}
	return &endpoints, nil
}

// FindAccent is configured with the parameters applied to a single metric
// for now, intensity is 1 for everything later on, intensity is used to
// 'weight' metrics i.e. give them greater harmonic meaning.
// Currently destination layer is not used
//
// i == Network index
// p == Metric name
func (q *QNet) FindAccent(p string, i int) *Accent {
	// The metric data
	md := q.Network[i].Mdata[p]

	// The metric max
	mx := q.Network[i].Maxval[p]

	// init values
	intensity := 1
	a := &Accent{}

	// if the accent exists, fill in a bunch of metadata
	if md >= mx {
		q.Network[i].Accent = make(map[string]*Accent)
		q.Network[i].Accent[p] = NewAccent(intensity, p)
		a = q.Network[i].Accent[p]

		slog.Debug("ACCENT FOUND",
			slog.Int64("Mdata", md),
			slog.Int64("Maxval", mx),
			slog.Int("Intensity", intensity),
			slog.Int64("Timestamp", a.Timestamp),
		)
	}

	return a
}

// Poll takes a string /p/ and searches QNet.Endpoint.Metric for the Key
// If the key is there, the metric is returned for that Key.
// This should not be needed use PollMulti()
func (q *QNet) Poll(p string) (int64, error) {
	// p is the Key to poll, it is a string
	// We know this is KV data right now, it's the only choice
	index := 0

	var metric int64
	metric = 0

	// poll is a map of KV
	poll, err := MetricKV(q.Network[index].URL)
	if err != nil {
		slog.Error("Could not poll metric", slog.Any("Error", err))
		return metric, err
	}

	// Search for the requested Key to poll /p/ in the configured list of keys
	for k := range poll {
		if k == p {
			// we've found the key, now grab its metric from the poll
			// convert the metric to int64 on assignment
			metric, err = strconv.ParseInt(poll[p], 10, 64)
			if err != nil {
				slog.Error("Could not convert metric to 64bits", slog.Any("Error", err))
				return metric, err
			}

			// Populate the map in the struct
			// Need to understand if maxval needs to be int64
			q.Network[index].Mdata[p] = metric

			/*
				// TODO: maxval must be set in the Endpoint struct for this key
				// maxval := q.Network[index].Maxval[p]
				// until then, use this
				maxval := 20

				// Did the measurement hit the accent maxval?
				accent := q.FindAccent(p, 0, maxval)
				if accent != nil {
					q.Network[index].Accent[p] = *accent
				}
			*/
		}
	}

	// should the raw metric or the accent metric be returned here?
	// maybe that can be a choice...
	return metric, nil
}

// PollMulti reads all configured metrics from QNet and retrieves them.
func (q *QNet) PollMulti() error {
	var metric int64
	metric = 0

	// Step through all Networks in QNet
	for ni, nv := range q.Network {
		// pollSource is a map of KV
		pollSource, err := MetricKV(q.Network[ni].URL)
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
