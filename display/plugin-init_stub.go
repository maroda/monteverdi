//go:build nomidi

package monteverdi

import (
	"fmt"
	"log/slog"
)

func InitMIDIOutput(view *View, outputLocation string) error {
	slog.Warn("MIDI support not compiled in this build")
	return fmt.Errorf("MIDI support not available")
}
