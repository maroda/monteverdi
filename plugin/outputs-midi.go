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

type MIDIOutput struct {
	Port drivers.Out
	Send func(msg midi.Message) error
	WG   sync.WaitGroup
}

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

	initmidi := &MIDIOutput{
		Port: out,
		Send: send,
		WG:   sync.WaitGroup{},
	}

	return initmidi, nil
}

func (mo *MIDIOutput) SendNoteOnMIDI(midic, midin, midiv uint8) error {
	return mo.Send(midi.NoteOn(midic, midin, midiv))
}

func (mo *MIDIOutput) SendNoteOffMIDI(midic, midin uint8) error {
	return mo.Send(midi.NoteOff(midic, midin))
}

func (mo *MIDIOutput) WritePulse(pulse *Mt.PulseEvent) error {
	var channel, note, velocity uint8
	channel = 0
	note = uint8(pulse.Pattern + 60)
	velocity = 100

	mo.WG.Add(1)
	go func() {
		defer mo.WG.Done()
		if err := mo.SendNoteOnMIDI(channel, note, velocity); err != nil {
			slog.Error("NoteOn event failed")
		}
		duration := pulse.Duration
		time.Sleep(duration)
		if err := mo.SendNoteOffMIDI(channel, note); err != nil {
			slog.Error("NoteOff event failed, attempting Flush")
			mo.Flush()
		}
	}()

	return nil
}

func (mo *MIDIOutput) Flush() error {
	return mo.Send(midi.ControlChange(0, midi.AllNotesOff, midi.Off))
}

func (mo *MIDIOutput) Close() error {
	mo.WG.Wait()

	if mo.Port != nil {
		mo.Port.Close()
		midi.CloseDriver()
	}
	return nil
}

func (mo *MIDIOutput) Type() string { return "MIDI" }
