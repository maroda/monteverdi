package monteverdi_test

import (
	"reflect"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

func TestNewQNet(t *testing.T) {
	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name, "https://popg.xyz")

	t.Run("Endpoint ID matches", func(t *testing.T) {
		// create a slice of Endpoint (also type:Endpoints)
		// using the Endpoint created above, just one element
		var eps []Ms.Endpoint
		eps = append(eps, *ep)

		// create a new QNet
		// check that the ID was created OK
		qn := Ms.NewQNet(eps)
		got := qn.Network[0].ID
		want := eps[0].ID
		assertString(t, got, want)
	})
}

func TestNewEndpoint(t *testing.T) {
	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name, "https://popg.xyz")
	id := ep.ID
	url := ep.URL

	// Struct literal
	want := struct {
		ID     string
		URL    string
		metric map[int64]string
		mdata  map[string]int64
	}{
		ID:     id,
		URL:    url,
		metric: ep.Metric,
		mdata:  ep.Mdata,
	}

	t.Run("Returns correct metadata", func(t *testing.T) {
		get := *Ms.NewEndpoint(id, url)
		got := get.URL
		match := want.URL
		assertString(t, got, match)

		got = get.ID
		match = want.ID
		assertString(t, got, match)
	})

	t.Run("Returns correct field count", func(t *testing.T) {
		got := *Ms.NewEndpoint(id, url, ep.Metric[1], ep.Metric[2], ep.Metric[3])
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewEndpoint returned wrong number of fields: got %d, want %d", gotSize, wantSize)
		}
	})

	t.Run("number of collections is correct", func(t *testing.T) {
		get := *Ms.NewEndpoint(id, url, ep.Metric[1], ep.Metric[2], ep.Metric[3])
		got := len(get.Metric)
		match := len(want.metric)
		if got != match {
			t.Errorf("NewEndpoint returned wrong number of collections: got %d, want %d", got, match)
		}
	})

}

// This may end up covering MetricKV but we'll see
func TestQNet_Poll(t *testing.T) {
	// create KV data
	kvbody := `VAR1=1
VAR2=11 # comment
VAR3=111
VAR4=1111

# A comment
VAR5=11111
`

	// create a mock web server
	mockWWW := makeMockWebServBody(0*time.Millisecond, kvbody)
	urlWWW := mockWWW.URL

	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name, urlWWW)

	// create a new QNet
	qn := Ms.NewQNet([]Ms.Endpoint{*ep})

	pollresult, err := qn.Poll("VAR3")
	if err != nil {
		t.Errorf("Poll returned unexpected error: %s", err)
	}

	// Here we look for VAR3
	t.Run("Fetches known KV", func(t *testing.T) {
		got := pollresult
		var want int64
		want = 111
		assertInt64(t, got, want)
	})
}

// Create an endpoint with a customizable ID and URL
// It contains three metrics and a data value for each metric
func makeEndpoint(i, u string) *Ms.Endpoint {
	// Fake ID
	id := i

	// Fake URL
	url := u

	// Collection map literal
	c := make(map[int64]string)
	c[1] = "ONE"
	c[2] = "TWO"
	c[3] = "THREE"

	// Collection data map literal
	d := make(map[string]int64)
	d[c[1]] = 1
	d[c[2]] = 2
	d[c[3]] = 3

	// Struct matches the Endpoint type
	return &Ms.Endpoint{
		ID:     id,
		URL:    url,
		Metric: c,
		Mdata:  d,
	}
}
