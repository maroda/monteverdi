package monteverdi

// QNet represents the entire connected Network of Qualities
// This is where it is configured which sources are being used
// And where pointers to the data are held

type Monteverdi interface {
	// QNet methods go here
	Init()
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
	metric map[int64]string // map of all metrics to be retrieved
	mdata  map[int64]int64  // map of all metric data synced by index
}

type Endpoints []Endpoint

// NewEndpoint returns a pointer to the Endpoint metadata and its data
// This function syncs endpoint with data using an index
func NewEndpoint(id, url string, m ...string) *Endpoint {
	collection := make(map[int64]string)
	colldata := make(map[int64]int64)

	// Keep the indexes synced by initializing them together
	index := int64(len(m) - 1)
	for _, value := range m {
		index++
		collection[index] = value
		colldata[index] = 0
	}
	return &Endpoint{
		ID:     id,
		URL:    url,
		metric: collection,
		mdata:  colldata,
	}
}

func (q *QNet) Init() {
	panic("implement me")
}

func (q *QNet) Poll(p string) {
	// p is the ID to poll
	// range q.Endpoints (this is a slice of Endpoint)
	// if the ID equals p, operate on that

	panic("implement me")
}
