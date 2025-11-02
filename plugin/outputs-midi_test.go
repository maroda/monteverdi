package plugin_test

import (
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Mt "github.com/maroda/monteverdi/types"
)

func TestMIDIOutput_WritePulse(t *testing.T) {
	now := time.Now()
	adapter, err := Mp.NewMIDIOutput(0)
	assertError(t, err, nil)
	defer adapter.Close()

	t.Run("Plays one simple note from a pulse", func(t *testing.T) {
		pulse := &Mt.PulseEvent{
			Dimension: 1,
			Metric:    []string{"NETWORK"},
			Pattern:   0,
			StartTime: now,
			Duration:  4*time.Second + 200*time.Millisecond,
		}

		adapter.WritePulse(pulse)
	})
}
