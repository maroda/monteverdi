package monteverdi_test

import (
	"fmt"
	"testing"
	"time"

	Mp "github.com/maroda/monteverdi/plugin"
	Ms "github.com/maroda/monteverdi/server"
	Mt "github.com/maroda/monteverdi/types"
)

/*
	BadgerOutput Adapter Plugin
	Monteverdi Server Tests

*/

func TestQNet_PulseDetectWriteBadgerDB(t *testing.T) {
	now := time.Now()
	qn := makeQNet(1)
	testMetric := "NETWORK"

	t.Run("Errors on BadgerDB error", func(t *testing.T) {
		// Use a custom failing output
		mock := &FailingBadgerOutput{ShouldFail: true}
		qn.Output = mock

		// Make sure we have a fresh sequence that will generate pulses
		qn.Network[0].Sequence[testMetric] = &Ms.IctusSequence{
			Metric: testMetric,
			Events: []Mt.Ictus{
				{Timestamp: now.Add(-15 * time.Second), IsAccent: false, Value: 5},
				{Timestamp: now.Add(-10 * time.Second), IsAccent: true, Value: 15},
				{Timestamp: now.Add(-5 * time.Second), IsAccent: false, Value: 8},
				{Timestamp: now, IsAccent: true, Value: 12},
			},
		}

		// Trigger pulse detection with testMetric, Endpoint 0
		// WritePulse will fail because DB is closed, but PulseDetect continues
		qn.PulseDetect(testMetric, 0)

		if mock.WritePulseCalls == 0 {
			t.Errorf("Expected to write badgerdb output but got nothing")
		}
	})

	t.Run("Writes values to BadgerDB during PulseDetect", func(t *testing.T) {
		// Define BadgerOutput adapter
		path := "./badger_db"
		batchSize := 1
		output, err := Mp.NewBadgerOutput(path, batchSize)
		defer output.Close()
		assertError(t, err, nil)
		qn.Output = output

		// Make sure we have a fresh sequence that will generate pulses
		qn.Network[0].Sequence[testMetric] = &Ms.IctusSequence{
			Metric: testMetric,
			Events: []Mt.Ictus{
				{Timestamp: now.Add(-15 * time.Second), IsAccent: false, Value: 5},
				{Timestamp: now.Add(-10 * time.Second), IsAccent: true, Value: 15},
				{Timestamp: now.Add(-5 * time.Second), IsAccent: false, Value: 8},
				{Timestamp: now, IsAccent: true, Value: 12},
			},
		}

		// Trigger pulse detection with testMetric, Endpoint 0
		qn.PulseDetect(testMetric, 0)

		// Now check if the database has the data
		got, err := qn.Output.QueryRange(now.Add(-20*time.Second), now.Add(2*time.Second))
		assertError(t, err, nil)
		assertStringContains(t, got[0].Metric[0], testMetric)
	})
}

// Helpers //

type FailingBadgerOutput struct {
	WritePulseCalls int
	ShouldFail      bool
	Pulses          []*Mt.PulseEvent
}

func (fbo *FailingBadgerOutput) WritePulse(pulse *Mt.PulseEvent) error {
	fbo.WritePulseCalls++
	if fbo.ShouldFail {
		return fmt.Errorf("mock write failure")
	}
	fbo.Pulses = append(fbo.Pulses, pulse)
	return nil
}
func (fbo *FailingBadgerOutput) WriteBatch(pulses []*Mt.PulseEvent) error {
	return nil
}
func (fbo *FailingBadgerOutput) QueryRange(start, end time.Time) ([]*Mt.PulseEvent, error) {
	return fbo.Pulses, nil
}
func (fbo *FailingBadgerOutput) Flush() error { return nil }
func (fbo *FailingBadgerOutput) Close() error { return nil }
func (fbo *FailingBadgerOutput) Type() string { return "FailingMock" }
