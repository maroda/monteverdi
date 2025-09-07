package monteverdi_test

import (
	"reflect"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

func TestNewAccent(t *testing.T) {
	want := struct {
		Timestamp time.Time
		Intensity int
		SourceID  string // identifies the output
		DestLayer *Ms.Timeseries
	}{
		Timestamp: time.Now(),
		Intensity: 1,
		SourceID:  "sourceID",
		DestLayer: &Ms.Timeseries{},
	}

	t.Run("Returns the correct number of fields", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID")
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewAccent returned incorrect number of fields, got: %d, want: %d", gotSize, wantSize)
		}
	})

	t.Run("Returns the correct IDs", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID")
		if got.SourceID != want.SourceID {
			t.Errorf("SourceID returned incorrect value, got: %s, want: %s", got.SourceID, want.SourceID)
		}
	})
}

func TestAccent_ValToRune(t *testing.T) {
	a := Ms.NewAccent(1, "craquemattic")
	runes := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	numset := []int64{1 - 1, 2 - 1, 3 - 1, 5 - 1, 8 - 1, 13 - 1, 21 - 1, 34}

	t.Run("Returns the correct rune for each metric value", func(t *testing.T) {
		for i, n := range numset {
			r := a.ValToRune(n)
			if r != runes[i] {
				t.Errorf("ValToRune returned incorrect value, got: %q, want: %q", r, runes[i])
			}
		}
	})
}

func TestAccent_AddSecond(t *testing.T) {
	a := Ms.NewAccent(1, "craquemattic")

	t.Run("Adds a metric/second and retrieves the correct rune", func(t *testing.T) {
		a.AddSecond(1.0)
		got := a.DestLayer.Runes[1]
		want := '▂'

		if got != want {
			t.Errorf("AddSecond returned incorrect value, got: %q, want: %q", got, want)
		}
	})
}

func TestAccent_GetDisplay(t *testing.T) {
	a := Ms.NewAccent(1, "craquemattic")

	t.Run("Returns the correct display value", func(t *testing.T) {
		// Without any data added, this should return 60 zeroes
		got := a.GetDisplay()
		want := []rune{
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetDisplay returned incorrect value, got: %q, want: %q", got, want)
		}
	})
}
