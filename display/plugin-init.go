//go:build !nomidi

package monteverdi

import (
	"fmt"
	"log/slog"

	Mp "github.com/maroda/monteverdi/plugin"
)

func InitMIDIOutput(view *View, outputLocation string) error {
	output, err := Mp.NewMIDIOutput(0)
	if err != nil {
		slog.Error("Failed to create adapter",
			slog.String("output", outputLocation),
			slog.Any("error", err))
		return err
	}
	view.QNet.Output = output
	slog.Info("MIDI Adapter Enabled", slog.String("output", outputLocation))
	return nil
}

func (v *View) getMIDISystemInfo(systemInfo *SystemInfo) {
	// If the output type is MIDI, fill in the details
	if midiOut, ok := v.QNet.Output.(*Mp.MIDIOutput); ok {
		systemInfo.MIDIPort = midiOut.Port.String()
		systemInfo.MIDIChannel = int(midiOut.Channel)
		systemInfo.MIDIRoot = int(midiOut.Root)
		systemInfo.MIDIScale = fmt.Sprint(midiOut.Scale)
		systemInfo.MIDINotes = fmt.Sprint(midiOut.ScNotes)
	}
}
