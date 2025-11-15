//go:build nomidi

package monteverdi

import "log/slog"

func (v *View) getMIDISystemInfo(systemInfo *SystemInfo) {
	slog.Warn("MIDI not supported on this platform")
}
