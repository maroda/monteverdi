package monteverdi

import (
	"reflect"
	"testing"
	"time"
)

// Test if the Timestamp is returning the correct format
// this hopefully runs in under 1s so the times should match
func TestAccentTime(t *testing.T) {
	got := *NewAccent(1, "main", "output")
	want := time.Now().Format("20060102T150405")

	if got.TimestampString() != want {
		t.Errorf("Accent.TimestampString() = %s, want %s", got.TimestampString(), want)
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
		got := *NewAccent(1, "sourceID", "destLayer")
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewAccent returned incorrect number of fields, got: %d, want: %d", gotSize, wantSize)
		}
	})

	t.Run("Returns the correct IDs", func(t *testing.T) {
		got := *NewAccent(1, "sourceID", "destLayer")
		if got.SourceID != want.SourceID {
			t.Errorf("SourceID returned incorrect value, got: %s, want: %s", got.SourceID, want.SourceID)
		}
		if got.DestLayer != want.DestLayer {
			t.Errorf("DestLayer returned incorrect value, got: %s, want: %s", got.DestLayer, want.DestLayer)
		}
	})

}
