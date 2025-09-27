package monteverdi_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	Ms "github.com/maroda/monteverdi/server"
)

func TestNewAccent(t *testing.T) {
	want := struct {
		Timestamp time.Time
		Intensity int
		SourceID  string // identifies the output
	}{
		Timestamp: time.Now(),
		Intensity: 1,
		SourceID:  "sourceID",
	}

	t.Run("Returns the correct number of fields", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID")
		gotSize := reflect.TypeOf(got).NumField()
		wantSize := reflect.TypeOf(want).NumField()
		if gotSize != wantSize {
			t.Errorf("NewAccent returned incorrect number of fields, got: %d, want: %d", gotSize, wantSize)
		}
	})

	t.Run("Returns the correct IDs", func(t *testing.T) {
		got := *Ms.NewAccent(1, "sourceID")
		if got.SourceID != want.SourceID {
			t.Errorf("SourceID returned incorrect value, got: %s, want: %s", got.SourceID, want.SourceID)
		}
	})
}

func TestNewPulseConfig(t *testing.T) {
	want := struct {
		IambStartPeriod    float64
		IambEndPeriod      float64
		TrocheeStartPeriod float64
		TrocheeEndPeriod   float64
	}{
		IambStartPeriod:    0.0,
		IambEndPeriod:      1.0,
		TrocheeStartPeriod: 0.0,
		TrocheeEndPeriod:   1.0,
	}

	t.Run("Returns the correct field assignments", func(t *testing.T) {
		config := Ms.NewPulseConfig(0.0, 1.0, 0.0, 1.0)
		if want.IambStartPeriod != config.IambStartPeriod {
			t.Errorf("IambStartPeriod returned incorrect value, got: %f, want: %f", config.IambStartPeriod, want.IambStartPeriod)
		}
		if want.IambEndPeriod != config.IambEndPeriod {
			t.Errorf("IambEndPeriod returned incorrect value, got: %f, want: %f", config.IambEndPeriod, want.IambEndPeriod)
		}
		if want.TrocheeStartPeriod != config.TrocheeStartPeriod {
			t.Errorf("TrocheeStartPeriod returned incorrect value, got: %f, want: %f", config.TrocheeStartPeriod, want.TrocheeStartPeriod)
		}
		if want.TrocheeEndPeriod != config.TrocheeEndPeriod {
			t.Errorf("TrocheeEndPeriod returned incorrect value, got: %f, want: %f", config.TrocheeEndPeriod, want.TrocheeEndPeriod)
		}
	})

}

func TestIctusSequence_DetectPulsesWithConfig(t *testing.T) {
	config := struct {
		IambStartPeriod    float64
		IambEndPeriod      float64
		TrocheeStartPeriod float64
		TrocheeEndPeriod   float64
	}{
		IambStartPeriod:    0.0,
		IambEndPeriod:      1.0,
		TrocheeStartPeriod: 0.0,
		TrocheeEndPeriod:   1.0,
	}

	t.Run("Returns Iamb", func(t *testing.T) {
		ictusSeq := &Ms.IctusSequence{
			Metric: "CPU1",
			Events: []Ms.Ictus{
				{Timestamp: time.Now(), IsAccent: false, Value: 45},
				{Timestamp: time.Now().Add(5 * time.Second), IsAccent: true, Value: 85},
				{Timestamp: time.Now().Add(10 * time.Second), IsAccent: false, Value: 50},
				{Timestamp: time.Now().Add(15 * time.Second), IsAccent: true, Value: 90},
			},
		}

		pulseEvent := ictusSeq.DetectPulsesWithConfig(config)

		for _, pulse := range pulseEvent {
			fmt.Printf("pulse in test: %v\n", pulse)
			fmt.Println(pulse.Pattern)
			if pulse.Pattern != Ms.Iamb {
				t.Errorf("Expected Iamb to be %v, got: %v", Ms.Iamb, pulse.Pattern)
			}
		}
	})

	t.Run("Returns Trochee", func(t *testing.T) {
		ictusSeq := &Ms.IctusSequence{
			Metric: "CPU1",
			Events: []Ms.Ictus{
				{Timestamp: time.Now(), IsAccent: true, Value: 85},
				{Timestamp: time.Now().Add(5 * time.Second), IsAccent: false, Value: 45},
				{Timestamp: time.Now().Add(10 * time.Second), IsAccent: true, Value: 90},
				{Timestamp: time.Now().Add(15 * time.Second), IsAccent: false, Value: 50},
			},
		}

		pulseEvent := ictusSeq.DetectPulsesWithConfig(config)

		for _, pulse := range pulseEvent {
			fmt.Printf("pulse in test: %v\n", pulse)
			fmt.Println(pulse.Pattern)
			if pulse.Pattern != Ms.Trochee {
				t.Errorf("Expected Iamb to be %v, got: %v", Ms.Trochee, pulse.Pattern)
			}
		}
	})
}

func TestIctusSequence_DetectPulses(t *testing.T) {
	qn := makeQNet(2)

	t.Run("Returns Iamb", func(t *testing.T) {
		tmetric := "CPU1"
		dSec1 := int64(90)
		dSec2 := int64(110)

		qn.Network[0].Maxval[tmetric] = 100

		// create data
		qn.Network[0].Mdata[tmetric] = dSec1
		// create []ictus
		qn.Network[0].RecordIctus(tmetric, false, dSec1)
		// create data
		qn.Network[0].Mdata[tmetric] = dSec2
		// create []ictus
		qn.Network[0].RecordIctus(tmetric, true, dSec2)

		sequence := qn.Network[0].Sequence[tmetric]
		pulseEvent := sequence.DetectPulses()

		for _, pulse := range pulseEvent {
			if pulse.Pattern != Ms.Iamb {
				t.Errorf("Did not detect Iamb: %d", pulse.Pattern)
			}
		}
	})

	t.Run("Returns Trochee", func(t *testing.T) {
		tmetric := "CPU2"
		dSec1 := int64(110)
		dSec2 := int64(90)

		qn.Network[1].Maxval[tmetric] = 100

		// create data
		qn.Network[1].Mdata[tmetric] = dSec1
		// create []ictus
		qn.Network[1].RecordIctus(tmetric, true, dSec1)
		// create data
		qn.Network[1].Mdata[tmetric] = dSec2
		// create []ictus
		qn.Network[1].RecordIctus(tmetric, false, dSec2)

		sequence := qn.Network[1].Sequence[tmetric]
		pulseEvent := sequence.DetectPulses()

		for _, pulse := range pulseEvent {
			if pulse.Pattern != Ms.Trochee {
				t.Errorf("Did not detect Trochee: %d", pulse.Pattern)
			}
		}
	})
}

func TestPulseSequence_DetectConsortPulses(t *testing.T) {
	testMetric := "CPU1"

	t.Run("Returns nil with an empty sequence", func(t *testing.T) {
		pulseSequence := &Ms.PulseSequence{
			Metric:    testMetric,
			Events:    []Ms.PulseEvent{},
			StartTime: time.Time{},
			EndTime:   time.Time{},
		}

		got := pulseSequence.DetectConsortPulses()
		if got != nil {
			t.Errorf("Expected nil, got: %v", got)
		}
	})

	t.Run("Returns nil with only one sequence", func(t *testing.T) {
		pulseSequence := &Ms.PulseSequence{
			Metric: testMetric,
			Events: []Ms.PulseEvent{
				{
					Dimension: 1,
					Pattern:   1,
					StartTime: time.Time{},
					Duration:  1,
					Metric:    []string{testMetric},
				},
			},
			StartTime: time.Time{},
			EndTime:   time.Time{},
		}

		got := pulseSequence.DetectConsortPulses()
		if got != nil {
			t.Errorf("Expected nil, got: %v", got)
		}
	})
}

func TestPulseSequence_TrimOldPulses(t *testing.T) {
	tg := &Ms.TemporalGrouper{
		PulseSequence: &Ms.PulseSequence{
			Events: []Ms.PulseEvent{
				{StartTime: time.Now().Add(-120 * time.Second)}, // 2 minutes old
				{StartTime: time.Now().Add(-90 * time.Second)},  // 1.5 minutes old
				{StartTime: time.Now().Add(-80 * time.Second)},  // 80 seconds old
			},
		},
	}

	// Set limit to 30s ago (all tests pulses are older than 30s)
	limiter := time.Now().Add(-30 * time.Second)

	tg.TrimBuffer(limiter)

	if len(tg.PulseSequence.Events) != 0 {
		t.Errorf("Expected 0, got %d events", len(tg.PulseSequence.Events))
	}

	if tg.PulseSequence.Events == nil {
		t.Errorf("Expected empty slice, got nil")
	}

}

func TestTemporalGrouper_HierarchyDetection(t *testing.T) {
	now := time.Now()
	testMetric := "CPU1"

	t.Run("Detects Amphibrach from Iamb→Trochee→Iamb sequence", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			WindowSize: 60 * time.Second,
			Buffer:     make([]Ms.PulseEvent, 0),
			Groups:     make([]*Ms.PulseTree, 0),
		}

		// Create the sequence that should trigger Amphibrach
		iamb1 := Ms.PulseEvent{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now,
			Duration:  5 * time.Second,
			Metric:    []string{testMetric},
		}

		trochee := Ms.PulseEvent{
			Dimension: 1,
			Pattern:   Ms.Trochee,
			StartTime: now.Add(6 * time.Second),
			Duration:  4 * time.Second,
			Metric:    []string{testMetric},
		}

		iamb2 := Ms.PulseEvent{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(11 * time.Second),
			Duration:  3 * time.Second,
			Metric:    []string{testMetric},
		}

		// Add the D1 pulses
		tg.AddPulse(iamb1)
		tg.AddPulse(trochee)
		tg.AddPulse(iamb2)

		// Check that we have both D1 and D2 pulses in buffer
		d1count := 0
		d2count := 0
		var amphibrachFound bool

		for _, pulse := range tg.Buffer {
			if pulse.Dimension == 1 {
				d1count++
			} else if pulse.Dimension == 2 {
				d2count++
				if pulse.Pattern == Ms.Amphibrach {
					amphibrachFound = true
				}
			}
		}

		// Should have 3 D1 pulses + 1 D2 pulse
		if d1count != 3 {
			t.Errorf("Expected 3 D1 pulses, got: %v", d1count)
		}

		if d2count != 1 {
			t.Errorf("Expected 1 D2 pulse, got: %v", d2count)
		}

		if !amphibrachFound {
			t.Errorf("Did not detect Amphibrach")
		}
	})

	t.Run("Detects multiple Amphibrachs from extended Iamb→Trochee sequence", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			WindowSize: 60 * time.Second,
			Buffer:     make([]Ms.PulseEvent, 0),
			Groups:     make([]*Ms.PulseTree, 0),
		}

		pulses := []Ms.PulseEvent{
			{Dimension: 1, Pattern: Ms.Iamb, StartTime: now, Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Trochee, StartTime: now.Add(3 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Iamb, StartTime: now.Add(6 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Trochee, StartTime: now.Add(9 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Iamb, StartTime: now.Add(12 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Trochee, StartTime: now.Add(15 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
			{Dimension: 1, Pattern: Ms.Iamb, StartTime: now.Add(18 * time.Second), Duration: 2 * time.Second, Metric: []string{testMetric}},
		}

		// Add all the D1 pulses
		for _, pulse := range pulses {
			tg.AddPulse(pulse)
		}

		// Count dimensions in buffer
		d1Count := 0
		d2Count := 0
		amphibrachCount := 0

		for _, pulse := range tg.Buffer {
			if pulse.Dimension == 1 {
				d1Count++
			} else if pulse.Dimension == 2 {
				d2Count++
				if pulse.Pattern == Ms.Amphibrach {
					amphibrachCount++
				}
			}
		}

		// t.Logf("D1 pulses: %d, D2 pulses: %d, Amphibrachs: %d", d1Count, d2Count, amphibrachCount)

		d1pulses := len(pulses)
		if d1Count != d1pulses {
			t.Errorf("Expected %d D1 pulses, got %d", d1pulses, d1Count)
		}

		// find at least one
		if amphibrachCount < 1 {
			t.Errorf("Expected at least 1 Amphibrach, got %d", amphibrachCount)
		}
	})
}

func TestTemporalGrouper_TrimBuffer(t *testing.T) {
	t.Run("Removes all pulses when none are after limit", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: time.Now().Add(-120 * time.Second)}, // 2 minutes ago
				{StartTime: time.Now().Add(-90 * time.Second)},  // 1.5 minutes ago
				{StartTime: time.Now().Add(-80 * time.Second)},  // 80 seconds ago
			},
		}

		// Set limit to 30 seconds ago - all pulses are older
		limit := time.Now().Add(-30 * time.Second)

		tg.TrimBuffer(limit)

		// Buffer should be completely empty
		if len(tg.Buffer) != 0 {
			t.Errorf("Did not remove all pulses")
		}
	})

	t.Run("Clears buffer when keepIndex equals buffer length", func(t *testing.T) {
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: time.Now().Add(-100 * time.Second)}, // Old pulse
				{StartTime: time.Now().Add(-80 * time.Second)},  // Old pulse
			},
		}

		// Set limit such that no pulses are after it
		limit := time.Now().Add(-10 * time.Second)

		tg.TrimBuffer(limit)

		// Should trigger the "Clear the buffer" path: tg.Buffer[:0]
		if len(tg.Buffer) != 0 {
			t.Errorf("Did not clear the buffer")
		}

		// Verify it's empty slice, not nil
		if tg.Buffer == nil {
			t.Errorf("Expected empty slice, got nil")
		}
	})

	t.Run("Keeps pulses that are after the limit", func(t *testing.T) {
		now := time.Now()
		tg := &Ms.TemporalGrouper{
			Buffer: []Ms.PulseEvent{
				{StartTime: now.Add(-80 * time.Second)}, // Too old
				{StartTime: now.Add(-70 * time.Second)}, // Too old
				{StartTime: now.Add(-20 * time.Second)}, // Keep this
				{StartTime: now.Add(-10 * time.Second)}, // Keep this
			},
		}

		limit := now.Add(-30 * time.Second)

		tg.TrimBuffer(limit)

		// Should keep only the last 2 pulses
		if len(tg.Buffer) != 2 {
			t.Errorf("Should have kept 2 pulses, got %d", len(tg.Buffer))
		}

		buff1 := tg.Buffer[0].StartTime.After(limit)
		buff2 := tg.Buffer[1].StartTime.After(limit)
		if !buff1 {
			t.Errorf("Should have been true, got %v", buff1)
		}
		if !buff2 {
			t.Errorf("Should have been true, got %v", buff2)
		}
	})
}

func TestTemporalGrouper_CreateGroupForPulses(t *testing.T) {
	tg := &Ms.TemporalGrouper{}
	pes := tg.Buffer

	t.Run("Returns nil for empty pulses", func(t *testing.T) {
		got := tg.CreateGroupForPulses(pes, 1)
		if got != nil {
			t.Errorf("Should have returned nil, got %v", got)
		}
	})
}

func TestTemporalGrouper_ProcessPendingPulses(t *testing.T) {
	// create a pending pulses - i.e. []PulseEvent
	// this goes into tg.PendingPulses
	// then check the tg.Buffer for the pulse
	// which means it was successfully processed

	testMetric := "CPU1"

	tg := &Ms.TemporalGrouper{}

	// One PulseEvent goes into pendingPulses
	pendingPulses := []Ms.PulseEvent{
		{
			Dimension: 1,
			Pattern:   0,
			StartTime: time.Now(),
			Duration:  1 * time.Second,
			Metric:    []string{testMetric},
		},
	}
	tg.PendingPulses = pendingPulses

	// Process should add the PulseEvent to the Buffer
	tg.ProcessPendingPulses()
	reflect.DeepEqual(tg.Buffer[0], pendingPulses[0])
}

func TestPulseEvents_FindChildren(t *testing.T) {
	now := time.Now()

	// Create parent amphibrach
	parentTime := now.Add(-10 * time.Second)

	// Create test data structure
	pulses := Ms.PulseEvents{
		// D1 children that belong to the parent
		{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-15 * time.Second),
			Parent:    parentTime, // Points to parent
		},
		{
			Dimension: 1,
			Pattern:   Ms.Trochee,
			StartTime: now.Add(-12 * time.Second),
			Parent:    parentTime, // Points to parent
		},
		{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-8 * time.Second),
			Parent:    parentTime, // Points to parent
		},
		// D2 parent amphibrach
		{
			Dimension: 2,
			Pattern:   Ms.Amphibrach,
			StartTime: parentTime,
			Parent:    time.Time{}, // Zero value = no parent
		},
		// Unrelated D1 pulse (different parent)
		{
			Dimension: 1,
			Pattern:   Ms.Iamb,
			StartTime: now.Add(-5 * time.Second),
			Parent:    now, // Different parent
		},
	}

	t.Run("Returns correct children", func(t *testing.T) {
		children := pulses.FindChildren(parentTime)

		want := 3
		got := len(children)
		assertInt(t, got, want)
	})

	t.Run("Returns empty for non-existent parent", func(t *testing.T) {
		nonExistentParent := now.Add(-100 * time.Second)
		children := pulses.FindChildren(nonExistentParent)

		want := 0
		got := len(children)
		assertInt(t, got, want)
	})
}

func TestAmphibrachTrimming(t *testing.T) {
	now := time.Now()
	tg := &Ms.TemporalGrouper{
		WindowSize: 60 * time.Second,
		Buffer: []Ms.PulseEvent{
			{
				Dimension: 2,
				Pattern:   Ms.Amphibrach,
				StartTime: now.Add(-45 * time.Second), // 45 seconds old
				Metric:    []string{"CPU1"},
			},
		},
	}

	// This should NOT remove the 45-second-old amphibrach
	limit := now.Add(-60 * time.Second)
	tg.TrimBuffer(limit)

	if len(tg.Buffer) == 0 {
		t.Error("45-second-old amphibrach was incorrectly trimmed")
	}
}

/// Helpers

func makePulsesWithGrouper() ([]Ms.PulseEvent, *Ms.TemporalGrouper) {
	grouper := &Ms.TemporalGrouper{
		WindowSize: 60 * time.Second,
		Buffer:     make([]Ms.PulseEvent, 0),
		Groups:     make([]*Ms.PulseTree, 0),
	}

	// Don't need a fake QNet here, just a representation of an IctusSequence
	ictusSeq := &Ms.IctusSequence{
		Metric: "CPU1",
		Events: []Ms.Ictus{
			{Timestamp: time.Now(), IsAccent: false, Value: 45},
			{Timestamp: time.Now().Add(5 * time.Second), IsAccent: true, Value: 85},
			{Timestamp: time.Now().Add(10 * time.Second), IsAccent: false, Value: 50},
			{Timestamp: time.Now().Add(15 * time.Second), IsAccent: true, Value: 90},
		},
	}

	// Collect the pulses from the ictus sequence
	return ictusSeq.DetectPulses(), grouper
}
