package plugin_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	Mp "github.com/maroda/monteverdi/plugin"
	Mt "github.com/maroda/monteverdi/types"
)

func TestNewBadgerOutput(t *testing.T) {
	adapter, closedb := makeTestBadgerOutput(t)
	compare := struct {
		db        *badger.DB
		batchSize int
		buffer    []*Mt.PulseEvent
	}{
		db:        adapter.DB,
		batchSize: 10,
		buffer:    make([]*Mt.PulseEvent, 0, 10),
	}
	defer closedb()

	t.Run("Creates new struct for output", func(t *testing.T) {
		path := "./badger_db"
		got, err := Mp.NewBadgerOutput(path, 10)
		assertError(t, err, nil)
		assertInt(t, got.BatchSize, compare.batchSize)
	})

	t.Run("Returns Type", func(t *testing.T) {
		want := "BadgerDB"
		got := adapter.Type()
		assertStringContains(t, got, want)
	})
}

func TestBadgerOutput_WritePulse(t *testing.T) {
	adapter, closedb := makeTestBadgerOutput(t)
	defer closedb()

	pulse := &Mt.PulseEvent{
		Dimension: 1,
		Pattern:   Mt.Iamb,
		StartTime: time.Now(),
		Duration:  5 * time.Second,
		Metric:    []string{"CPU"},
	}

	t.Run("Writes pulse without error", func(t *testing.T) {
		err := adapter.WritePulse(pulse)
		assertError(t, err, nil)
	})

	t.Run("Flushes pulses for writing", func(t *testing.T) {
		start := time.Now()
		// the test adapter buffer size is 5
		pulses := []*Mt.PulseEvent{
			{Dimension: 1, Pattern: 0, StartTime: start},
			{Dimension: 1, Pattern: 1, StartTime: start.Add(1 * time.Second)},
			{Dimension: 1, Pattern: 0, StartTime: start.Add(2 * time.Second)},
			{Dimension: 1, Pattern: 1, StartTime: start.Add(3 * time.Second)},
			{Dimension: 1, Pattern: 0, StartTime: start.Add(4 * time.Second)},
		}

		// Send all pulses
		for _, p := range pulses {
			err := adapter.WritePulse(p)
			assertError(t, err, nil)
		}

		// Verify database entries
		var readPulses []*Mt.PulseEvent
		readPulses, err := adapter.QueryRange(start.Add(-1*time.Second), start.Add(5*time.Second))
		assertError(t, err, nil)

		// Verify Count
		if len(readPulses) != len(pulses) {
			t.Errorf("Expected %d pulses, got %d", len(pulses), len(readPulses))
		}

		// Verify data match
		if len(readPulses) > 0 {
			if readPulses[0].Dimension != pulses[0].Dimension {
				t.Errorf("Dimension mismatch: got %d, want %d", readPulses[0].Dimension, pulses[0].Dimension)
			}
			if readPulses[0].Pattern != pulses[0].Pattern {
				t.Errorf("Pulse Pattern mismatch: got %d, want %d", readPulses[0].Pattern, pulses[0].Pattern)
			}
		}
	})
}

func TestBadgerOutput_PulseKeyValue(t *testing.T) {
	pulse := &Mt.PulseEvent{
		Dimension: 1,
		Pattern:   Mt.Iamb,
		StartTime: time.Now(),
		Duration:  5 * time.Second,
		Metric:    []string{"NETWORK"},
	}

	t.Run("Makes a Pulse Key", func(t *testing.T) {
		key := make([]byte, 8+1+len(pulse.Metric[0]))
		t.Logf("key: %s", key)

		// The last five bytes will be: CPU00
		want := make([]byte, 5)
		mb := []byte("NETWORK")
		copy(want, mb[:5])
		t.Logf("want: %s", want)
		t.Logf("mb: %s", mb)

		get := Mp.PulseKey(pulse)
		t.Logf("get: %v", get)

		got := get[9:]
		t.Logf("got: %v", got)

		if !bytes.Equal(want, got) {
			t.Errorf("PulseKey = %v, want %v", got, want)
		}
	})
}

func TestBadgerOutput_WriteBatch(t *testing.T) {
	tests := []struct {
		name    string
		pulses  []*Mt.PulseEvent
		wantErr bool
	}{
		{
			name:    "empty batch",
			pulses:  []*Mt.PulseEvent{},
			wantErr: false,
		},
		{
			name: "single pulse",
			pulses: []*Mt.PulseEvent{
				{Dimension: 1, Pattern: 0, StartTime: time.Now()},
			},
			wantErr: false,
		},
		{
			name: "multiple pulses",
			pulses: []*Mt.PulseEvent{
				{Dimension: 1, Pattern: 0, StartTime: time.Now()},
				{Dimension: 1, Pattern: 1, StartTime: time.Now().Add(1 * time.Second)},
				{Dimension: 1, Pattern: 0, StartTime: time.Now().Add(2 * time.Second)},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter, closedb := makeTestBadgerOutput(t)
			defer closedb()

			err := adapter.WriteBatch(tt.pulses)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteBatch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBadgerOutput_QueryRange(t *testing.T) {
	adapter, closedb := makeTestBadgerOutput(t)
	defer closedb()

	t.Run("QueryRange returns values", func(t *testing.T) {
		start := time.Now()
		pulses := []*Mt.PulseEvent{
			{Dimension: 1, Pattern: 0, StartTime: start},
			{Dimension: 1, Pattern: 1, StartTime: start.Add(1 * time.Second)},
			{Dimension: 1, Pattern: 0, StartTime: start.Add(2 * time.Second)},
			{Dimension: 1, Pattern: 1, StartTime: start.Add(3 * time.Second)},
			{Dimension: 1, Pattern: 0, StartTime: start.Add(4 * time.Second)},
		}

		// Send all pulses
		for _, p := range pulses {
			err := adapter.WritePulse(p)
			assertError(t, err, nil)
		}

		var queryResults []*Mt.PulseEvent
		queryResults, err := adapter.QueryRange(start.Add(-1*time.Second), start.Add(5*time.Second))
		assertError(t, err, nil)

		for _, qr := range queryResults {
			t.Logf("QueryResult StartTime: %v", qr.StartTime)
		}

		if len(queryResults) != len(pulses) {
			t.Errorf("Expected %d results, got %d", len(pulses), len(queryResults))
		}
	})
}

// Helpers //

func makeTestBadgerOutput(t *testing.T) (*Mp.BadgerOutput, func()) {
	t.Helper()

	opts := badger.DefaultOptions("").WithInMemory(true).WithLogger(nil)
	db, err := badger.Open(opts)
	assertError(t, err, nil)

	adapter := &Mp.BadgerOutput{
		DB:        db,
		BatchSize: 5,
		Buffer:    make([]*Mt.PulseEvent, 0, 5),
	}

	cleanup := func() {
		adapter.Close()
	}

	return adapter, cleanup
}
