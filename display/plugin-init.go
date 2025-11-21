//go:build !nomidi

package monteverdi

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
)

func InitMIDIOutput(view *View, outputLocation string) error {
	midiPort := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_PORT", 0)
	midiRoot := uint8(Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ROOT", 60))
	midiArpD := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ARP_DELAY", 300)
	midiArpI := Ms.FillEnvVarInt("MONTEVERDI_PLUGIN_MIDI_ARP_INTERVAL", 1)
	midiScale := Ms.FillEnvVar("MONTEVERDI_PLUGIN_MIDI_SCALE")

	slog.Info("Configuration found:",
		slog.Int("Port", midiPort),
		slog.Any("Root", midiRoot),
		slog.Int("ArpDelay", midiArpD),
		slog.Int("Interval", midiArpI),
		slog.String("Scale", midiScale),
	)

	var scaleI []uint8
	var scaleS []string
	scaleS = strings.Split(midiScale, ",")
	for _, v := range scaleS {
		interval, err := strconv.Atoi(v)
		if err != nil {
			slog.Error("Could not read MIDI_SCALE value, using default", slog.Any("error", err), slog.String("value", v))
			scaleI = []uint8{0, 2, 2, 1, 2, 2, 2, 1}
			break
		}
		scaleI = append(scaleI, uint8(interval))
	}

	output, err := Mp.NewMIDIOutput(midiPort, midiArpD, midiArpI, midiRoot, scaleI)
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
