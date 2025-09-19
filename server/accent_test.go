package monteverdi_test

import (
	"fmt"
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

/*
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
*/

func TestIctusSequence_DetectPulses(t *testing.T) {
	qn := makeQNet(2)

	t.Run("Returns Iamb", func(t *testing.T) {
		tmetric := "CPU1"
		dSec1 := int64(90)
		dSec2 := int64(110)

		qn.Network[0].Maxval[tmetric] = 100

		// create data
		qn.Network[0].Mdata[tmetric] = dSec1
		// create []ictus
		qn.Network[0].RecordIctus(tmetric, false, dSec1)
		// create data
		qn.Network[0].Mdata[tmetric] = dSec2
		// create []ictus
		qn.Network[0].RecordIctus(tmetric, true, dSec2)

		sequence := qn.Network[0].Sequence[tmetric]
		pulseEvent := sequence.DetectPulses()

		for _, pulse := range pulseEvent {
			if pulse.Pattern != Ms.Iamb {
				t.Errorf("Did not detect Iamb: %d", pulse.Pattern)
			}
		}
	})

	t.Run("Returns Trochee", func(t *testing.T) {
		tmetric := "CPU2"
		dSec1 := int64(110)
		dSec2 := int64(90)

		qn.Network[1].Maxval[tmetric] = 100

		// create data
		qn.Network[1].Mdata[tmetric] = dSec1
		// create []ictus
		qn.Network[1].RecordIctus(tmetric, true, dSec1)
		// create data
		qn.Network[1].Mdata[tmetric] = dSec2
		// create []ictus
		qn.Network[1].RecordIctus(tmetric, false, dSec2)

		sequence := qn.Network[1].Sequence[tmetric]
		pulseEvent := sequence.DetectPulses()

		for _, pulse := range pulseEvent {
			if pulse.Pattern != Ms.Trochee {
				t.Errorf("Did not detect Trochee: %d", pulse.Pattern)
			}
		}
	})
}

func makePulsesWithGrouper() ([]Ms.PulseEvent, *Ms.TemporalGrouper) {
	grouper := &Ms.TemporalGrouper{
		WindowSize: 60 * time.Second,
		Buffer:     make([]Ms.PulseEvent, 0),
		Groups:     make([]*Ms.PulseTree, 0),
	}

	// Don't need a fake QNet here, just a representation of an IctusSequence
	ictusSeq := &Ms.IctusSequence{
		Metric: "CPU1",
		Events: []Ms.Ictus{
			{Timestamp: time.Now(), IsAccent: false, Value: 45},
			{Timestamp: time.Now().Add(5 * time.Second), IsAccent: true, Value: 85},
			{Timestamp: time.Now().Add(10 * time.Second), IsAccent: false, Value: 50},
			{Timestamp: time.Now().Add(15 * time.Second), IsAccent: true, Value: 90},
		},
	}

	// Collect the pulses from the ictus sequence
	return ictusSeq.DetectPulses(), grouper
}

func TestTemporalGrouper_AddPulse(t *testing.T) {
	pulses, grouper := makePulsesWithGrouper()

	t.Run("Adds correct number of pulses to a group", func(t *testing.T) {
		for _, pulse := range pulses {
			grouper.AddPulse(pulse)

			// Print which pattern was detected
			switch pulse.Pattern {
			case Ms.Iamb:
				fmt.Printf("Detected Iamb at %v, duration: %v\n", pulse.StartTime, pulse.Duration)
			case Ms.Trochee:
				fmt.Printf("Detected Trochee at %v, duration: %v\n", pulse.StartTime, pulse.Duration)
			}
		}

		// First check that just one group was created
		got := len(grouper.Groups)
		want := 1
		assertInt(t, got, want)

		// Now check that there are three in the group
		for i, group := range grouper.Groups {
			count := len(group.Pulses)
			if count != want*3 {
				t.Errorf("Group %d: Expected %d pulses, got %d", i, want*3, count)
			}
			// fmt.Printf("Group %d: %d pulses over %v\n", i, len(group.Pulses), group.Duration)
		}
	})

	// Use the timing from the literal ictus sequence above
	t.Run("Registers correct timing for pulses", func(t *testing.T) {
		for _, pulse := range pulses {
			grouper.AddPulse(pulse)
			expectedSec := int64(5)
			actualSec := int64(pulse.Duration / time.Second)
			if expectedSec != actualSec {
				t.Errorf("Expected: %d, got: %d", expectedSec, actualSec)
			}
		}
	})
}

func TestTemporalGrouper_TrimBuffer(t *testing.T) {
	t.Run("Removes all pulses when none are after limit", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: time.Now().Add(-120 * time.Second)}, // 2 minutes ago
				{StartTime: time.Now().Add(-90 * time.Second)},  // 1.5 minutes ago
				{StartTime: time.Now().Add(-80 * time.Second)},  // 80 seconds ago
			},
		}

		// Set limit to 30 seconds ago - all pulses are older
		limit := time.Now().Add(-30 * time.Second)

		tg.TrimBuffer(limit)

		// Buffer should be completely empty
		if len(tg.Buffer) != 0 {
			t.Errorf("Did not remove all pulses")
		}
	})

	t.Run("Clears buffer when keepIndex equals buffer length", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: time.Now().Add(-100 * time.Second)}, // Old pulse
				{StartTime: time.Now().Add(-80 * time.Second)},  // Old pulse
			},
		}

		// Set limit such that no pulses are after it
		limit := time.Now().Add(-10 * time.Second)

		tg.TrimBuffer(limit)

		// Should trigger the "Clear the buffer" path: tg.Buffer[:0]
		if len(tg.Buffer) != 0 {
			t.Errorf("Did not clear the buffer")
		}

		// Verify it's empty slice, not nil
		if tg.Buffer == nil {
			t.Errorf("Expected empty slice, got nil")
		}
	})

	t.Run("Keeps pulses that are after the limit", func(t *testing.T) {
		now := time.Now()
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: now.Add(-80 * time.Second)}, // Too old
				{StartTime: now.Add(-70 * time.Second)}, // Too old
				{StartTime: now.Add(-20 * time.Second)}, // Keep this
				{StartTime: now.Add(-10 * time.Second)}, // Keep this
			},
		}

		limit := now.Add(-30 * time.Second)

		tg.TrimBuffer(limit)

		// Should keep only the last 2 pulses
		if len(tg.Buffer) != 2 {
			t.Errorf("Should have kept 2 pulses, got %d", len(tg.Buffer))
		}

		buff1 := tg.Buffer[0].StartTime.After(limit)
		buff2 := tg.Buffer[1].StartTime.After(limit)
		if !buff1 {
			t.Errorf("Should have been true, got %v", buff1)
		}
		if !buff2 {
			t.Errorf("Should have been true, got %v", buff2)
		}
	})
}
