//go:build !nomidi

package monteverdi

import (
	"fmt"

	Mp "github.com/maroda/monteverdi/plugin"
)

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
