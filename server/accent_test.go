package monteverdi_test

import (
	"reflect"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

// Test if the Timestamp is returning the correct format
func TestAccentTime(t *testing.T) {
	got := *Ms.NewAccent(1, "main", "output")
	want := time.Now().UnixNano()

	if got.Timestamp != want {
		t.Errorf("Accent = %q, want %q", got, want)
	}
}

func TestNewAccent(t *testing.T) {
	want := struct {
		Timestamp           time.Time
		Intensity           int
		SourceID, DestLayer string // identifies the output
	}{
		Timestamp: time.Now(),
		Intensity: 1,
		SourceID:  "sourceID",
		DestLayer: "destLayer",
	}

	t.Run("Returns the correct number of fields", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID", "destLayer")
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewAccent returned incorrect number of fields, got: %d, want: %d", gotSize, wantSize)
		}
	})

	t.Run("Returns the correct IDs", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID", "destLayer")
		if got.SourceID != want.SourceID {
			t.Errorf("SourceID returned incorrect value, got: %s, want: %s", got.SourceID, want.SourceID)
		}
		if got.DestLayer != want.DestLayer {
			t.Errorf("DestLayer returned incorrect value, got: %s, want: %s", got.DestLayer, want.DestLayer)
		}
	})

}
