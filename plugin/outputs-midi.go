package plugin

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	Mt "github.com/maroda/monteverdi/types"
	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
	_ "gitlab.com/gomidi/midi/v2/drivers/rtmididrv"
)

// MIDIOutput is a
type MIDIOutput struct {
	WG       sync.WaitGroup
	Port     drivers.Out                    // standard driver type
	Send     func(msg midi.Message) error   // any midi message
	NoteOff  func(midic, midin uint8) error // e.g. for quantization and testing
	Channel  uint8                          // MIDI Channel, 0-15
	Root     uint8                          // Root MIDI note (C3 = 60)
	Velocity uint8                          // MIDI note velocity (0-127)
	Scale    []uint8                        // Scale Intervals starting from 0 (for Root)
	ScIdx    int                            // Track Scale Index for interval interpolation
}

// NewMIDIOutput is the router for pulse to become MIDI note,
// this is where the `rtmididrv` is initiated and devices connected.
// This also contains metadata for the musical notes being played.
func NewMIDIOutput(port int) (*MIDIOutput, error) {
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

	defaultRoot := uint8(60)
	defaultVelocity := uint8(100)
	defaultScale := []uint8{0, 2, 2, 1, 2, 2, 2, 1} // Diatonic major

	initmidi := &MIDIOutput{
		WG:       sync.WaitGroup{},
		Port:     out,
		Send:     send,
		Channel:  0,
		Root:     defaultRoot,
		Velocity: defaultVelocity,
		Scale:    defaultScale,
		ScIdx:    0,
	}
	initmidi.NoteOff = initmidi.SendNoteOffMIDI

	return initmidi, nil
}

// SendNoteOnMIDI is the bridge between WritePulse and MIDIOutput
func (mo *MIDIOutput) SendNoteOnMIDI(midic, midin, midiv uint8) error {
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

// WritePulse takes one PulseEvent and translates it to a MIDI event
func (mo *MIDIOutput) WritePulse(pulse *Mt.PulseEvent) error {
	// ScaleStep takes the root note and returns the note to play.
	// Each time it's called, it increases the interface index counter,
	// so that the next note played will be the next note in the scale.
	channel := mo.Channel
	note := mo.Root
	velocity := mo.Velocity

	if nn := mo.ScaleStep(mo.Root); nn != 0 {
		note = nn
	}

	if err := mo.SendNoteOnMIDI(channel, note, velocity); err != nil {
		slog.Error("NoteOn event failed")
		return fmt.Errorf("NoteOn event failed: %q", err)
	}

	// NoteOffMIDI is performed in a goroutine, which allows
	// notes to be independently stacked.
	mo.WG.Add(1)
	go func() {
		defer mo.WG.Done()
		duration := pulse.Duration
		time.Sleep(duration)
		if err := mo.NoteOff(channel, note); err != nil {
			slog.Error("NoteOff event failed, attempting Flush")
			mo.Flush()
		}
	}()

	return nil
}

// Flush sends an AllNotesOff to the active channel
// TODO: that channel should be from the struct
func (mo *MIDIOutput) Flush() error {
	return mo.Send(midi.ControlChange(0, midi.AllNotesOff, midi.Off))
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

// WriteBatch is Not Implemented
func (mo *MIDIOutput) WriteBatch(pulses []*Mt.PulseEvent) error {
	return fmt.Errorf("not implemented")
}

// QueryRange is Not Implemented
func (mo *MIDIOutput) QueryRange(start, end time.Time) ([]*Mt.PulseEvent, error) {
	return nil, fmt.Errorf("not implemented")
}
