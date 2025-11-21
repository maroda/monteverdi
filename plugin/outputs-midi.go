//go:build !nomidi

package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	Mt "github.com/maroda/monteverdi/types"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var (
	DiatonicMajor = []uint8{0, 2, 2, 1, 2, 2, 2, 1}
	NaturalMinor  = []uint8{0, 2, 1, 2, 2, 2, 1, 2}
)

// MIDIOutput is the interface for the MIDI Output Plugin Adapter
type MIDIOutput struct {
	WG       sync.WaitGroup                 // Go channels for NoteOff events
	QueMU    sync.Mutex                     // Note Tracker Queue protection
	Queue    []ScheduledNote                // Pending notes being tracked
	Port     drivers.Out                    // standard driver type
	Send     func(msg midi.Message) error   // any midi message
	NoteOff  func(midic, midin uint8) error // e.g. for quantization and testing
	Channel  uint8                          // MIDI Channel, 0-15
	Root     uint8                          // Root MIDI note (C3 = 60)
	Velocity uint8                          // MIDI note velocity (0-127)
	Scale    []uint8                        // Scale Intervals starting from 0 (for Root)
	ScIdx    int                            // Track Scale Index for interval interpolation
	ScNotes  []uint8                        // Notes computed from Root and Scale
	ArpSpace int                            // Build arpeggios with equal timing
	IntSpace int                            // Build chords with WriteBatch using larger intervals
	QLimit   int                            // Quantize note limit for one beat
	IsPoly   bool                           // Whether the MIDI output is polyphonic (false: monophonic)
	Grouper  []*Mt.PulseEvent               // Grouper for chords or other entities
	LastTS   time.Time                      // Most recent pulse timestamp
}

// ScheduledNote is a tracking queue used for reporting purposes
// It sits in parallel with the real-time MIDI note management
type ScheduledNote struct {
	Channel uint8
	Note    uint8
	OffTime time.Time // NoteOff is fired
}

// NewMIDIOutput is the router for pulse to become MIDI note,
// this is where the `rtmididrv` is initiated and devices connected.
// This also contains metadata for the musical notes being played.
func NewMIDIOutput(port, arpd, arpi int, root uint8, scale []uint8) (*MIDIOutput, error) {
	ctx := context.Background()
	ctx, span := otel.Tracer("monteverdi/plugin").Start(ctx, "NewMIDIOutput")
	defer span.End()

	out, err := midi.OutPort(port)
	if err != nil {
		slog.Error("Error opening MIDI port", slog.Int("port", port))
		return nil, fmt.Errorf("error opening MIDI port: %q", err)
	}

	send, err := midi.SendTo(out)
	if err != nil {
		slog.Error("Error sending to MIDI port", slog.Int("port", port))
		return nil, fmt.Errorf("error sending to MIDI port: %q", err)
	}

	defaultArp := arpd
	defaultInt := arpi
	defaultRoot := root
	defaultScale := scale
	defaultChannel := uint8(0)
	defaultVelocity := uint8(100)

	initmidi := &MIDIOutput{
		WG:       sync.WaitGroup{},
		QueMU:    sync.Mutex{},
		Queue:    []ScheduledNote{},
		Port:     out,
		Send:     send,
		Channel:  defaultChannel,
		Root:     defaultRoot,
		Velocity: defaultVelocity,
		Scale:    defaultScale,
		ScIdx:    0,
		ArpSpace: defaultArp,
		IntSpace: defaultInt,
		QLimit:   4,
		IsPoly:   false,
		Grouper:  []*Mt.PulseEvent{},
		LastTS:   time.Now(),
	}
	initmidi.NoteOff = initmidi.SendNoteOffMIDI
	initmidi.ScNotes = initmidi.ScaleNotes()

	slog.Info("Created MIDI output",
		slog.Any("available.ports", midi.GetOutPorts()),
		slog.String("configured.port", initmidi.Port.String()),
		slog.Int("send.channel", int(initmidi.Channel)),
		slog.Int("root", int(initmidi.Root)),
		slog.String("scale", fmt.Sprint(initmidi.Scale)),
		slog.String("scale.notes", fmt.Sprint(initmidi.ScNotes)),
	)

	return initmidi, nil
}

// ScaleNotes returns the computed scale from Root plus Scale intervals
// It will not overwrite any existing scale.f
func (mo *MIDIOutput) ScaleNotes() []uint8 {
	if mo.ScNotes == nil {
		mo.ComputeScaleNotes()
	}
	return mo.ScNotes
}

// ComputeScaleNotes takes the configured Root and Scale
// and builds the actual scale with MIDI notes starting from Root.
func (mo *MIDIOutput) ComputeScaleNotes() {
	note := uint8(0)       // Root Note
	mo.ScNotes = []uint8{} // Empty Scale of (no) Notes
	mo.ScIdx = 0           // Start with the first index (Root Note)

	// Step through the Scale and build the Notes
	for i := 0; i < len(mo.Scale); i++ {
		// When nn is 0 we're starting on Root, so don't step
		if nn := mo.ScaleStep(mo.Root); nn != 0 {
			note = nn
		}
		mo.ScNotes = append(mo.ScNotes, note)
	}
}

// SendNoteOnMIDI is the bridge between WritePulse and MIDIOutput
func (mo *MIDIOutput) SendNoteOnMIDI(midic, midin, midiv uint8) error {
	slog.Debug("MIDI NoteOn",
		slog.Int("note", int(midin)),
		slog.Int("velocity", int(midiv)),
		slog.Time("time", time.Now()))
	return mo.Send(midi.NoteOn(midic, midin, midiv))
}

// SendNoteOffMIDI is used to stop the note
func (mo *MIDIOutput) SendNoteOffMIDI(midic, midin uint8) error {
	return mo.Send(midi.NoteOff(midic, midin))
}

// ScaleStep takes a Root note and derives the next note value
// based on a sequence of intervals defined in Scale.
func (mo *MIDIOutput) ScaleStep(root uint8) uint8 {
	// Build the note number by adding up all notes in the scale up to this index
	notes := uint8(0)
	for i := 0; i < len(mo.Scale); i++ {
		notes = notes + mo.Scale[i] // Add the number of intervals at Scale[i]
		if i == mo.ScIdx {          // When i reaches the Scale Index, that's the last note to add
			break
		}
	}

	// Prep the index for the next run
	// This enables wrap-around of the notes in the defined interval series
	// So to get something like two octaves, mo.Scale will need to be two octaves
	if mo.ScIdx == len(mo.Scale)-1 {
		// When the Scale Index has read all members of mo.Scale, reset to 0
		// the next note will be the first note above the root
		mo.ScIdx = 0
	} else {
		// Otherwise, increase
		// the next note will be the next interval above the current note
		mo.ScIdx++
	}
	slog.Debug("NEXT scaling step", slog.Int("step", mo.ScIdx))
	slog.Debug("Root note", slog.Int("root", int(root)))
	slog.Debug("Current note", slog.Int("notes", int(root+notes)))

	return root + notes
}

// WriteBatch builds a chord from closely timed pulses
// and plays a stacked chord based on ScNotes.
// When Polyphony is set with IsPoly it plays a chord,
// and breaks it into an arpeggio in default Monophonic mode.
func (mo *MIDIOutput) WriteBatch(pulses []*Mt.PulseEvent) error {
	ctx := context.Background()
	ctx, span := otel.Tracer("monteverdi/plugin").Start(ctx, "WriteBatch")
	defer span.End()

	// The /range/ here will complete for each note within microseconds
	// which should be fast enough to appear as a chord in MIDI
	mo.WG.Add(1)
	go func() error {
		defer mo.WG.Done()

		for i, pulse := range pulses {
			slog.Debug("Interval Spacing", slog.Int("space", mo.IntSpace))
			noteIdx := (mo.IntSpace * i) % len(mo.ScNotes)
			note := mo.ScNotes[noteIdx]

			// If monophonic, space out the notes into an arpeggio
			if !mo.IsPoly {
				time.Sleep(time.Duration(mo.ArpSpace) * time.Millisecond)
			}

			if err := mo.SendNoteOnMIDI(mo.Channel, note, mo.Velocity); err != nil {
				slog.Error("WriteBatch NoteOn event failed")
				return fmt.Errorf("WriteBatch NoteOn event failed: %q", err)
			}

			// NoteOffMIDI is performed in a goroutine,
			// allowing notes to be independently stacked.
			// Before the WaitGroup is created, the NoteOff
			// data is shared with a tracking queue.
			noteOffTime := time.Now().Add(pulse.Duration)
			mo.QueMU.Lock()
			mo.Queue = append(mo.Queue, ScheduledNote{
				Channel: mo.Channel,
				Note:    note,
				OffTime: noteOffTime,
			})
			mo.QueMU.Unlock()

			mo.WG.Add(1)
			go func(duration time.Duration, note uint8, offTime time.Time) {
				defer mo.WG.Done()

				_, noteSpan := otel.Tracer("monteverdi/plugin").Start(ctx, "WriteBatch.NoteOff_goroutine")
				defer noteSpan.End()

				time.Sleep(duration)

				// Note is removed from tracker queue
				// before NoteOff event is sent
				mo.QueMU.Lock()
				mo.Queue = trimNoteTracker(mo.Queue, note, offTime)
				mo.QueMU.Unlock()

				if err := mo.NoteOff(mo.Channel, note); err != nil {
					slog.Error("NoteOff event failed, attempting Flush")
					mo.Flush()
				}
			}(pulse.Duration, note, noteOffTime)
		}
		return nil
	}()

	return nil
}

type ChordGrouper struct {
	WindowSize time.Duration
	Buffer     []Mt.PulseEvent
}

// WritePulse takes one PulseEvent and translates it to a MIDI event
func (mo *MIDIOutput) WritePulse(pulse *Mt.PulseEvent) error {
	ctx := context.Background()
	ctx, span := otel.Tracer("monteverdi/plugin").Start(ctx, "WritePulse")
	defer span.End()

	span.SetAttributes(
		attribute.String("midi.output.port", mo.Port.String()),
		attribute.Int("pulse.pattern", int(pulse.Pattern)),
		attribute.Int64("pulse.duration_ms", pulse.Duration.Milliseconds()),
	)

	// This is initialized explicitly. If a note comes >50ms after
	// the preceding note, it becomes the first qualifier for the next
	// potential chord of notes within 50ms of each other, in other words
	// it is added as the first member of the Grouper. First members
	// must wait to see if the next note is under 50ms or not. Then
	// the note is either played then (the next note is >50ms away)
	// or saved for the chord (at which point there are 2 in the group).
	var play bool
	if len(mo.Grouper) == 0 {
		// So do not play it immediately,
		play = false
	} else {
		play = true
	}

	if len(mo.Grouper) > 0 && pulse.StartTime.Sub(mo.LastTS) < 50*time.Millisecond {
		play = false
		mo.Grouper = append(mo.Grouper, pulse)
		slog.Debug("Grouping pulse", slog.Int("pulse", len(mo.Grouper)))
	} else {
		// at this point the grouper is either 0 or the next note is not within 50ms
		// this will not be reached until all simultaneous notes are added to the chord
		if len(mo.Grouper) > 1 {
			play = false
			slog.Debug("Playing batch")
			if err := mo.WriteBatch(mo.Grouper); err != nil {
				slog.Error("WriteBatch event failed", slog.Any("error", err))
				return fmt.Errorf("WriteBatch failure: %q", err)
			}
		} else if len(mo.Grouper) == 1 {
			slog.Debug("Playing single")
		}
		// Reaching this far means a Grouper has been exhausted,
		// the next note > 50ms was played,
		// and so the current note is the first of the next potential chord
		mo.Grouper = []*Mt.PulseEvent{pulse}
	}

	if play {
		// ScaleStep takes the root note and returns the note to play.
		// Each time it's called, it increases the interface index counter,
		// so that the next note played will be the next note in the scale.
		note := mo.Root
		if nn := mo.ScaleStep(mo.Root); nn != 0 {
			note = nn
		}

		if err := mo.SendNoteOnMIDI(mo.Channel, note, mo.Velocity); err != nil {
			slog.Error("NoteOn event failed")
			return fmt.Errorf("NoteOn event failed: %q", err)
		}

		// NoteOffMIDI is performed in a goroutine,
		// allowing notes to be independently stacked.
		// Before the WaitGroup is created, the NoteOff
		// data is shared with a tracking queue.
		noteOffTime := time.Now().Add(pulse.Duration)
		mo.QueMU.Lock()
		mo.Queue = append(mo.Queue, ScheduledNote{
			Channel: mo.Channel,
			Note:    note,
			OffTime: noteOffTime,
		})
		mo.QueMU.Unlock()

		mo.WG.Add(1)
		go func(duration time.Duration, note uint8, offTime time.Time) {
			defer mo.WG.Done()

			// The OTEL child span is set here for measuring the entire note event.
			_, noteSpan := otel.Tracer("monteverdi/plugin").Start(ctx, "WritePulse.NoteOff_goroutine")
			defer noteSpan.End()

			time.Sleep(duration)

			// Note is removed from tracker queue
			// before NoteOff event is sent
			mo.QueMU.Lock()
			mo.Queue = trimNoteTracker(mo.Queue, note, offTime)
			mo.QueMU.Unlock()

			if err := mo.NoteOff(mo.Channel, note); err != nil {
				slog.Error("NoteOff event failed, attempting Flush")
				mo.Flush()
			}
		}(pulse.Duration, note, noteOffTime)

	}

	mo.LastTS = pulse.StartTime
	return nil
}

func trimNoteTracker(queue []ScheduledNote, note uint8, offTime time.Time) []ScheduledNote {
	for i, sn := range queue {
		if sn.Note == note && sn.OffTime.Equal(offTime) {
			return append(queue[:i], queue[i+1:]...)
		}
	}
	return queue
}

// Flush sends an AllNotesOff to the active channel
func (mo *MIDIOutput) Flush() error {
	// Reset the Note Tracker
	mo.QueMU.Lock()
	mo.Queue = []ScheduledNote{}
	mo.QueMU.Unlock()

	// Send AllNotesOff
	return mo.Send(midi.ControlChange(mo.Channel, midi.AllNotesOff, midi.Off))
}

// Close will Wait until all NoteOff routines are completed and then close the device
func (mo *MIDIOutput) Close() error {
	mo.WG.Wait()

	if mo.Port != nil {
		mo.Port.Close()
		midi.CloseDriver()
	}
	return nil
}

// Type for this plugin
func (mo *MIDIOutput) Type() string { return "MIDI" }

// QueryRange reads the Note Tracker Queue to return information about future notes.
func (mo *MIDIOutput) QueryRange(start, end time.Time) (interface{}, error) {
	mo.QueMU.Lock()
	defer mo.QueMU.Unlock()

	if len(mo.Queue) == 0 {
		return &NoteTracker{Depth: 0}, nil
	}

	// Locate queue bookends by finding min/max of NoteOff time
	oldest := mo.Queue[0].OffTime
	newest := mo.Queue[0].OffTime
	for _, sn := range mo.Queue {
		if sn.OffTime.Before(oldest) {
			oldest = sn.OffTime
		}
		if sn.OffTime.After(newest) {
			newest = sn.OffTime
		}
	}

	return &NoteTracker{
		Depth:          len(mo.Queue),
		Oldest:         oldest,
		Newest:         newest,
		Window:         newest.Sub(oldest),
		GrouperSize:    len(mo.Grouper),
		ActiveRoutines: runtime.NumGoroutine(),
	}, nil
}

// NoteTracker is a Queue for tracking future notes.
type NoteTracker struct {
	Depth          int           `json:"noteDepth"`
	Oldest         time.Time     `json:"noteOldest"`
	Newest         time.Time     `json:"noteNewest"`
	Window         time.Duration `json:"noteWindow"`
	GrouperSize    int           `json:"noteGrouperSize"`
	ActiveRoutines int           `json:"activeRoutines"`
}
