package monteverdi

import (
	"reflect"
	"testing"
)

func TestNewQNet(t *testing.T) {

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
