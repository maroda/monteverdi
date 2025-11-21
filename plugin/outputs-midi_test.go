package plugin_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Mt "github.com/maroda/monteverdi/types"
	"gitlab.com/gomidi/midi/v2"
)

func TestMIDIOutput_QueryRange(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
	assertError(t, err, nil)
	defer adapter.Close()

	t.Run("Returns Empty QueryRange", func(t *testing.T) {
		get, err := adapter.QueryRange(time.Now(), time.Now())
		assertError(t, err, nil)
		got, ok := get.(*Mp.NoteTracker)
		if !ok {
			t.Errorf("Expected Mp.NoteTracker, got %T", get)
		}

		depth := 0
		// The defined depth should equal the constructed queue depth
		assertInt(t, depth, got.Depth)
	})

	t.Run("Returns QueryRange with NoteTracker", func(t *testing.T) {
		// Add to the Note Tracker Queue
		depth := 8
		for i := 0; i < depth; i++ {
			adapter.Queue = append(adapter.Queue, Mp.ScheduledNote{
				Channel: adapter.Channel,
				Note:    adapter.ScNotes[i],
				OffTime: time.Now().Add(time.Duration((i*100)+rand.Intn(200)) * time.Millisecond),
			})
		}

		get, err := adapter.QueryRange(time.Now(), time.Now())
		assertError(t, err, nil)
		got, ok := get.(*Mp.NoteTracker)
		if !ok {
			t.Errorf("Expected Mp.NoteTracker, got %T", get)
		}

		// The defined depth should equal the constructed queue depth
		assertInt(t, depth, got.Depth)
	})
}

func TestMIDIOutput_Type(t *testing.T) {
	t.Run("Returns Type", func(t *testing.T) {
		adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
		assertError(t, err, nil)
		defer adapter.Close()

		if adapter.Type() != "MIDI" {
			t.Error("Expected MIDI for Type()")
		}
	})
}

func TestMIDIOutput_ScaleStep(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
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

func TestMIDIOutput_ScaleNotes(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
	assertError(t, err, nil)
	defer adapter.Close()

	t.Run("Returns correct scale", func(t *testing.T) {
		CMajor := []uint8{60, 62, 64, 65, 67, 69, 71, 72}
		// DiatonicMaj := []uint8{0, 2, 2, 1, 2, 2, 2, 1}

		got := adapter.ScaleNotes()
		if !reflect.DeepEqual(got, CMajor) {
			t.Errorf("Expected ScaleNotes to return %v, got %v", CMajor, got)
		}
	})
}

// INTEGRATION TESTING: Audio confirmation of tests is recommended //

func TestMIDIOutput_WritePulse(t *testing.T) {
	t.Run("Errors when NoteOn fails", func(t *testing.T) {
		// Use the MIDI adapter on the first port
		adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
		assertError(t, err, nil)
		defer adapter.Close()

		origSend := adapter.Send
		// inject a SendNoteOnMIDI error
		adapter.Send = func(msg midi.Message) error {
			return fmt.Errorf("SendNoteOnMIDI error")
		}
		defer func() { adapter.Send = origSend }()

		pulse1 := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now(),
			Duration:  100 * time.Millisecond,
		}

		pulse2 := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now().Add(500 * time.Millisecond),
			Duration:  100 * time.Millisecond,
		}

		// SendNoteOnMIDI is the first function for WritePulse
		// the expected behavior is to return an error
		// if SendNoteOnMIDI fails
		err = adapter.WritePulse(pulse1)
		err = adapter.WritePulse(pulse2) // Followup note is needed to trigger the player
		assertGotError(t, err)
		adapter.WG.Wait()
	})

	t.Run("Flush method is correctly called", func(t *testing.T) {
		// Use the MIDI adapter on the first port
		adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
		assertError(t, err, nil)
		defer adapter.Close()

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

		pulse1 := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now(),
			Duration:  1 * time.Second,
		}

		pulse2 := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: time.Now().Add(500 * time.Millisecond),
			Duration:  1 * time.Second,
		}

		adapter.WritePulse(pulse1)
		adapter.WritePulse(pulse2)
		adapter.WG.Wait()

		if !flushCalled {
			t.Error("Expected flush to be called when NoteOff fails")
		}

		adapter.SendNoteOffMIDI(0, 62)
	})
}

func TestMIDIOutput_ScaleAudio(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
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

func TestMIDIOutput_WriteBatch(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
	assertError(t, err, nil)
	defer adapter.Close()

	tests := []struct {
		name   string
		pulses []*Mt.PulseEvent
		space  int
		poly   bool
	}{
		{
			name:  "Plays an arpeggio major chord",
			space: 2,
			poly:  false,
			pulses: []*Mt.PulseEvent{
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1500 * time.Millisecond,
				},
			},
		},
		{
			name:  "Plays wrapped arpeggio",
			space: 7,
			poly:  false,
			pulses: []*Mt.PulseEvent{
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  500 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1500 * time.Millisecond,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.IsPoly = tt.poly
			adapter.IntSpace = tt.space
			adapter.WriteBatch(tt.pulses)
			adapter.WG.Wait()
		})
	}

	// Flush any remaining grouped pulses
	if len(adapter.Grouper) > 0 {
		adapter.WriteBatch(adapter.Grouper)
	}
}

func TestMIDIOutput_WritePulseBatch(t *testing.T) {
	adapter, err := Mp.NewMIDIOutput(0, 300, 1, uint8(60), Mp.DiatonicMajor)
	assertError(t, err, nil)
	defer adapter.Close()

	tests := []struct {
		name   string
		pulses []*Mt.PulseEvent
		space  int
		poly   bool
		arp    int
	}{
		{
			name:  "Plays an arpeggio major chord",
			space: 2,
			poly:  false,
			arp:   500,
			pulses: []*Mt.PulseEvent{
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now(),
					Duration:  1000 * time.Millisecond,
				},
				{
					Dimension: 1,
					Metric:    []string{"NETWORK"},
					Pattern:   0,
					StartTime: time.Now().Add(500 * time.Millisecond),
					Duration:  1000 * time.Millisecond,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.ArpSpace = tt.arp
			adapter.IsPoly = tt.poly
			adapter.IntSpace = tt.space
			for i := range tt.pulses {
				adapter.WritePulse(tt.pulses[i])
			}
		})
	}

	// Flush any remaining grouped pulses
	if len(adapter.Grouper) > 0 {
		adapter.WriteBatch(adapter.Grouper)
	}
}
