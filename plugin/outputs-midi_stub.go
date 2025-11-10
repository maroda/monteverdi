//go:build nomidi

package plugin

import (
	"fmt"
	"time"

	Mt "github.com/maroda/monteverdi/types"
)

type MIDIOutput struct{}

func (m *MIDIOutput) WritePulse(pulse *Mt.PulseEvent) error {
	return fmt.Errorf("MIDI support not compiled in this build")
}

func (m *MIDIOutput) WriteBatch(pulses []*Mt.PulseEvent) error {
	return fmt.Errorf("MIDI support not compiled in this build")
}

func (m *MIDIOutput) QueryRange(start, end time.Time) ([]*Mt.PulseEvent, error) {
	return nil, fmt.Errorf("MIDI support not compiled in this build")
}

func (m *MIDIOutput) Flush() error { return nil }
func (m *MIDIOutput) Close() error { return nil }
func (m *MIDIOutput) Type() string { return "midi-disabled" }
