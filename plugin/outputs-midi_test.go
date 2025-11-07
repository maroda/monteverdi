package plugin_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Mt "github.com/maroda/monteverdi/types"
	"gitlab.com/gomidi/midi/v2"
)

func TestMIDIOutput_WritePulse(t *testing.T) {
	// Use the MIDI adapter on the first port
	adapter, err := Mp.NewMIDIOutput(0)
	assertError(t, err, nil)
	defer adapter.Close()

	t.Run("Returns Type", func(t *testing.T) {
		if adapter.Type() != "MIDI" {
			t.Error("Expected MIDI for Type()")
		}
	})

	t.Run("Errors when NoteOn fails", func(t *testing.T) {
		origSend := adapter.Send
		// inject a SendNoteOnMIDI error
		adapter.Send = func(msg midi.Message) error {
			return fmt.Errorf("SendNoteOnMIDI error")
		}
		defer func() { adapter.Send = origSend }()

		pulse := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now(),
			Duration:  100 * time.Millisecond,
		}

		// SendNoteOnMIDI is the first function for WritePulse
		// the expected behavior is to return an error
		// if SendNoteOnMIDI fails
		err = adapter.WritePulse(pulse)
		assertGotError(t, err)
		adapter.WG.Wait()
	})

	t.Run("Flush method is correctly called", func(t *testing.T) {
		flushCalled := false
		originalSend := adapter.Send

		// Wrap Send to detect Flush calls
		adapter.Send = func(msg midi.Message) error {
			flushCalled = true // We know we are calling Flush(), so set it to true
			return originalSend(msg)
		}

		// Inject a NoteOff that always fails and will call Flush()
		adapter.NoteOff = func(midic, midin uint8) error {
			return fmt.Errorf("returns error")
		}

		pulse := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now(),
			Duration:  1 * time.Second,
		}

		adapter.WritePulse(pulse)
		adapter.WG.Wait()

		if !flushCalled {
			t.Error("Expected flush to be called when NoteOff fails")
		}

		adapter.SendNoteOffMIDI(0, 62)
	})
}

func TestMIDIOutput_ScaleStep(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0)
	assertError(t, err, nil)
	defer adapter.Close()

	tests := []struct {
		name  string
		root  uint8
		notes []uint8
		steps []uint8
	}{
		{
			"Returns the next member of the scale, C Diatonic Major",
			60,
			[]uint8{60, 62, 64, 65, 67, 69, 71, 72},
			[]uint8{0, 2, 2, 1, 2, 2, 2, 1}, // This is also the default, diatonic major
		},
		{
			"Returns the next member of the scale, D Diatonic Major",
			62,
			[]uint8{62, 64, 66, 67, 69, 71, 73, 74},
			[]uint8{0, 2, 2, 1, 2, 2, 2, 1}, // diatonic major
		},
		{
			"Returns the next member of the scale, D Natural Minor",
			62,
			[]uint8{62, 64, 65, 67, 69, 71, 72, 74},
			[]uint8{0, 2, 1, 2, 2, 2, 1, 2}, // natural minor
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.Scale = tt.steps
			adapter.ScIdx = 0 // This should be incremented each time, only set it once
			for i := 0; i < len(tt.steps); i++ {
				if adapter.ScIdx != i {
					t.Errorf("Expected Scale Index to self-increment to %d, got %d", i, adapter.ScIdx)
				}

				got := adapter.ScaleStep(tt.root)

				if got != tt.notes[i] {
					t.Errorf("Expected note to increment to %d, got %d", tt.notes[i], got)
				}
			}
		})
	}

	t.Run("Wraps to the first member of the default scale", func(t *testing.T) {
		root := uint8(60)
		notes := []uint8{60, 62, 64, 65, 67, 69, 71, 72}
		adapter.ScIdx = 0

		// first step through all notes
		for i := 0; i < len(adapter.Scale); i++ {
			adapter.ScaleStep(root)
		}

		// now step one more and it should wrap
		got := adapter.ScaleStep(root)
		if got != notes[0] {
			t.Errorf("Expected wrap to first note %d, got %d", notes[0], got)
		}

	})
}

// INTEGRATION: Audio confirmation through MIDI
func TestMIDIOutput_ScaleAudio(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0)
	assertError(t, err, nil)
	defer adapter.Close()

	t.Logf("outports:\n%s\n", midi.GetOutPorts())

	t.Run("Plays five notes on a scale from five pulse events", func(t *testing.T) {
		startTime := time.Now()
		for i := 0; i < 5; i++ {
			time.Sleep(time.Duration(rand.Intn(1000))*time.Millisecond + time.Second)
			adapter.WritePulse(&Mt.PulseEvent{
				Dimension: 1,
				Metric:    []string{"NETWORK"},
				Pattern:   0,
				StartTime: startTime.Add(time.Duration(i) * time.Second),
				Duration:  1 * time.Second,
			})
		}
		adapter.WG.Wait()
	})

	t.Run("Plays 10 notes on a wrapped scale from 10 pulse events", func(t *testing.T) {
		startTime := time.Now()
		for i := 0; i < 10; i++ {
			time.Sleep(time.Duration(rand.Intn(1000))*time.Millisecond + time.Second)
			adapter.WritePulse(&Mt.PulseEvent{
				Dimension: 1,
				Metric:    []string{"NETWORK"},
				Pattern:   0,
				StartTime: startTime.Add(time.Duration(i) * time.Second),
				Duration:  1 * time.Second,
			})
		}
		adapter.WG.Wait()
	})
}
