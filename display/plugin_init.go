//go:build !nomidi

package monteverdi

import (
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
