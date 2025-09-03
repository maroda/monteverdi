package monteverdi

import (
	"reflect"
	"testing"
)

func TestNewQNet(t *testing.T) {
	// create a new Endpoint
	name := "craquemattic"
	ep := makeEndpoint(name)

	t.Run("Endpoint ID matches", func(t *testing.T) {
		// create a slice of Endpoint (also type:Endpoints)
		// using the Endpoint created above, just one element
		var eps []Endpoint
		eps = append(eps, *ep)

		// create a new QNet
		// check that the ID was created OK
		qn := NewQNet(eps)
		got := qn.Network[0].ID
		want := eps[0].ID
		assertString(t, got, want)
	})
}

func TestNewEndpoint(t *testing.T) {
	// Fake ID
	id := "craquemattic"

	// Fake URL
	url := "https://popg.xyz"

	// Collection map literal
	c := make(map[int64]string)
	c[1] = "ONE"
	c[2] = "TWO"
	c[3] = "THREE"

	// Collection data map literal
	d := make(map[int64]int64)
	d[1] = 1
	d[2] = 2
	d[3] = 3

	// Struct literal
	want := struct {
		ID     string
		URL    string
		metric map[int64]string
		mdata  map[int64]int64
	}{
		ID:     id,
		URL:    url,
		metric: c,
		mdata:  d,
	}

	t.Run("Returns correct metadata", func(t *testing.T) {
		get := *NewEndpoint(id, url)
		got := get.URL
		match := want.URL
		assertString(t, got, match)

		got = get.ID
		match = want.ID
		assertString(t, got, match)
	})

	t.Run("Returns correct field count", func(t *testing.T) {
		got := *NewEndpoint(id, url, c[1], c[2], c[3])
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewEndpoint returned wrong number of fields: got %d, want %d", gotSize, wantSize)
		}
	})

	t.Run("number of collections is correct", func(t *testing.T) {
		get := *NewEndpoint(id, url, c[1], c[2], c[3])
		got := len(get.metric)
		match := len(want.metric)
		if got != match {
			t.Errorf("NewEndpoint returned wrong number of collections: got %d, want %d", got, match)
		}
	})

}

func makeEndpoint(i string) *Endpoint {
	// Fake ID
	id := i

	// Fake URL
	url := "https://popg.xyz"

	// Collection map literal
	c := make(map[int64]string)
	c[1] = "ONE"
	c[2] = "TWO"
	c[3] = "THREE"

	// Collection data map literal
	d := make(map[int64]int64)
	d[1] = 1
	d[2] = 2
	d[3] = 3

	// Struct matches the Endpoint type
	return &Endpoint{
		ID:     id,
		URL:    url,
		metric: c,
		mdata:  d,
	}
}
