package monteverdi

import (
	"log/slog"
	"strconv"
)

// QNet represents the entire connected Network of Qualities
// This is where it is configured which sources are being used
// And where pointers to the data are held

type Monteverdi interface {
	// QNet methods go here
	Poll()
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

// The idea here is that the app will take this Endpoint
// check the URL for whatever comes back
// and then use each metric listed in the map to grab data
//
// 1. app starts, reads config file. maybe this is TOML, or
//    another idea would be to have an API that takes a JSON config
// 2. this config is read into a slice of Endpoint entries
// 3. This slice - Endpoints - is fed into the machine
// 4. The QNet struct contains pointers to the data itself

type Endpoint struct {
	ID     string           // string describing the endpoint source
	URL    string           // URL endpoint for the service
	Metric map[int64]string // map of all metric keys to be retrieved
	Mdata  map[string]int64 // map of all metric data by metric key
}

type Endpoints []Endpoint

// NewEndpoint returns a pointer to the Endpoint metadata and its data
// This function syncs endpoint with data using an index
func NewEndpoint(id, url string, m ...string) *Endpoint {
	collection := make(map[int64]string)
	colldata := make(map[string]int64)

	// index for the metric collection
	index := int64(len(m) - 1)
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

// Poll takes a string /p/ and searches the Endpoint for the Key
// If the key is there, the metric is returned for that Key.
func (q *QNet) Poll(p string) (int64, error) {
	// p is the Key to poll, it is a string

	// We know this is KV data right now, it's the only choice
	// this probably also needs to operate on each member of the slice
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
			q.Network[index].Mdata[p] = metric
		}
	}

	return metric, nil
}
