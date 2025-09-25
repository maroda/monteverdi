package monteverdi

/*

This file is loosely arranged in the order of data operations from granular to macro.
They look a little like this:

An Accent is a measurement above a threshold.
An Ictus is a change where a threshold is crossed or exited.
A Group is at least three events formed from an IctusSequence.
A Pulse is defined across a Group.
A Pulse Pattern is a specific literary pattern: Iamb, Trochee, Amphibrach, etc.
A Pulse Event is an instance of a Pulse Pattern.
The PulseTree contains a hierarchy of detected Pulse Patterns.
A Dimension is a layer of the PulseTree hierarchy.
An IctusSequence is in Dimension 1.
A PulseSequence is in Dimension 2.
A Consort is in D2: at least three pulses from a PulseSequence , i.e. a group of Groups.
There are no Consort Patterns, they are all Pulse Patterns.
Pulse Events are produced by both Groups and Consorts.
The TemporalGrouper is the heart of the algorithm that groups things together.

*/

import (
	"log/slog"
	"time"
)

// The Accent is the building block of this tool.
type Accent struct {
	Timestamp int64  // Unix timestamp TODO: this should be time.Time
	Intensity int    // raw, unweighted accent strength
	SourceID  string // identifies the source
}

// Timeseries is a generic fixed TimeSeries DB
type Timeseries struct {
	Runes   []rune
	MaxSize int
	Current int
}

// NewAccent builds the metadata for the accent
// There is no boolean, the existence of an Accent is always true
func NewAccent(i int, s string) *Accent {
	return &Accent{
		Timestamp: time.Now().UnixNano(),
		Intensity: i,
		SourceID:  s,
	}
}

type Ictus struct {
	Timestamp time.Time
	IsAccent  bool
	Value     int64
	Duration  time.Duration
}

type IctusSequence struct {
	Metric    string
	Events    []Ictus
	StartTime time.Time
	EndTime   time.Time
}

// These trigrams are read top-down, representing an accent/non-accent sequence
const (
	axnt       = "⚊" // U+268B is an accent (yang)
	noax       = "⚋" // U+268A is a non-accent (yin)
	iamb       = "⚍" // U+268D is noax, axnt (lesser yin)
	anapest    = "☳" // U+2633 is noax, noax, axnt (thunder)
	trochee    = "⚎" // U+268E is axnt, noax (lesser yang)
	dactyl     = "☶" // U+2636 is axnt, noax, noax (mountain)
	amphibrach = "☵" // U+2635 is noax, axnt, noax (water)
)

// PulsePattern is the seed of Meyer pattern matching
// But why pulse?
// Meyer explicitly points out the difference between rhythm and pulse.
// Accents define the pulse of something, which are separate from its regular rhythms.
// For example, the expected rhythm of an operational system can include health checks,
// garbage collection, predictable load, etc. But the pulse represents a deeper, more
// fundamental force as the system interacts with the real-world, as its rhythms continue.
type PulsePattern int

// PulsePattern entries
// Iamb: non-accent → accent
// Trochee: accent → non-accent
// Amphibrach: non-accent → accent → non-accent (not yet implemented)
const (
	Iamb PulsePattern = iota
	Trochee
	Amphibrach
	Anapest
	Dactyl
)

type PulseEvent struct {
	Dimension    int
	Pattern      PulsePattern
	StartTime    time.Time
	Duration     time.Duration
	Metric       []string
	Significance float64
}

type PulseConfig struct {
	IambStartPeriod    float64 // 0.0 = start of non-accent, 0.5 = middle, 1.0 = end
	IambEndPeriod      float64 // 0.0 = start of accent, 0.5 = middle, 1.0 = end
	TrocheeStartPeriod float64 // For accent period
	TrocheeEndPeriod   float64 // For non-accent period
}

// NewPulseConfig returns a set of parameters
// used to define pulse pattern periodicity
func NewPulseConfig(is, ie, ts, te float64) *PulseConfig {
	return &PulseConfig{
		IambStartPeriod:    is,
		IambEndPeriod:      ie,
		TrocheeStartPeriod: ts,
		TrocheeEndPeriod:   te,
	}
}

// DetectPulsesWithConfig takes the ictus sequence and
// recognizes two patterns that make up a pulse:
// Iamb has no accent followed by an accent,
// Trochee has an accent followed by no accent.
func (is *IctusSequence) DetectPulsesWithConfig(config PulseConfig) []PulseEvent {
	var pulses []PulseEvent

	// We need at least three events to process
	if len(is.Events) < 3 {
		return pulses
	}

	for i := 1; i < len(is.Events)-2; i++ {
		prev := is.Events[i-1]
		curr := is.Events[i]
		next := is.Events[i+1]

		// Iamb pattern: non-accent → accent transition
		if !prev.IsAccent && curr.IsAccent {
			nonAccentDur := curr.Timestamp.Sub(prev.Timestamp)
			accentDur := next.Timestamp.Sub(curr.Timestamp)

			// Configurable START within the non-accent period
			patternStart := prev.Timestamp.Add(time.Duration(float64(nonAccentDur) * config.IambStartPeriod))

			// Configurable END within the non-accent period
			patternEnd := curr.Timestamp.Add(time.Duration(float64(accentDur) * config.IambEndPeriod))

			pulses = append(pulses, PulseEvent{
				Dimension: 1,
				Pattern:   Iamb,
				StartTime: patternStart,
				Duration:  patternEnd.Sub(patternStart),
				Metric:    []string{is.Metric},
			})
		}

		// Trochee pattern: accent → non-accent transition
		if prev.IsAccent && !curr.IsAccent {
			accentDur := curr.Timestamp.Sub(prev.Timestamp)
			nonAccentDur := next.Timestamp.Sub(curr.Timestamp)

			patternStart := prev.Timestamp.Add(time.Duration(float64(accentDur) * config.TrocheeStartPeriod))
			patternEnd := curr.Timestamp.Add(time.Duration(float64(nonAccentDur) * config.TrocheeEndPeriod))

			pulses = append(pulses, PulseEvent{
				Dimension: 1,
				Pattern:   Trochee,
				StartTime: patternStart,
				Duration:  patternEnd.Sub(patternStart),
				Metric:    []string{is.Metric},
			})
		}
	}

	return pulses
}

// DetectPulses takes the ictus sequence and
// recognizes two patterns that make up a pulse:
// Iamb has no accent followed by an accent,
// Trochee has an accent followed by no accent.
// TODO: this version of the algorithm is broken
func (is *IctusSequence) DetectPulses() []PulseEvent {
	var pulses []PulseEvent

	// Take all ictus events and determine what we're seeing
	for i := 0; i < len(is.Events)-1; i++ {
		curr := is.Events[i]
		next := is.Events[i+1]

		// Detect Iamb: non-accent → accent
		if !curr.IsAccent && next.IsAccent {
			pulses = append(pulses, PulseEvent{
				Dimension: 1,
				Pattern:   Iamb,
				StartTime: curr.Timestamp,
				Duration:  next.Timestamp.Sub(curr.Timestamp),
				Metric:    []string{is.Metric},
			})
		}

		// Detect Trochee: accent → non-accent
		if curr.IsAccent && !next.IsAccent {
			pulses = append(pulses, PulseEvent{
				Dimension: 1,
				Pattern:   Trochee,
				StartTime: curr.Timestamp,
				Duration:  next.Timestamp.Sub(curr.Timestamp),
				Metric:    []string{is.Metric},
			})
		}
	}
	return pulses
}

type PulseTree struct {
	Dimension int
	Pulses    []PulsePattern // The constituent pulses
	OGEvents  []PulseEvent   // Preserve source event data
	StartTime time.Time
	Duration  time.Duration
	Frequency int             // How often this grouping occurs
	Children  []*PulseTree    // Lower-level patterns that comprise this one
	VizData   []PulseVizPoint // Generic visualization descriptor
}

type PulseVizPoint struct {
	Position  int          // position on the timeline
	Pattern   PulsePattern // Iamb or Trochee
	IsAccent  bool         // Is an accent
	Duration  time.Duration
	StartTime time.Time
	Extends   bool // pattern extends beyond display
}

type PulseSequence struct {
	Metric    string
	Events    []PulseEvent
	StartTime time.Time
	EndTime   time.Time
}

// DetectConsortPulses takes the pulses in a PulseSequence
// and creates higher-dimension pulses from lower-dimension ones
func (ps *PulseSequence) DetectConsortPulses() []PulseEvent {
	var consort []PulseEvent

	// Need at least 3 pulses to detect patterns
	if len(ps.Events) < 3 {
		return consort
	}

	for i := 0; i < len(ps.Events)-2; i++ {
		first := ps.Events[i]
		second := ps.Events[i+1]
		third := ps.Events[i+2]

		// Detect Amphibrach: Iamb → Trochee → Iamb
		// (non-accent→accent) → (accent→non-accent) → (non-accent→accent)
		if first.Pattern == Iamb && second.Pattern == Trochee && third.Pattern == Iamb {
			newD2Pulse := PulseEvent{
				Dimension: 2,
				Pattern:   Amphibrach,
				StartTime: first.StartTime,
				Duration:  third.StartTime.Add(third.Duration).Sub(first.StartTime),
				Metric:    first.Metric,
			}
			consort = append(consort, newD2Pulse)

			slog.Info("NEW CONSORT PATTERN", slog.Any("event", newD2Pulse))
		}

		// Detect Anapest: Iamb → Iamb → Trochee

		// Detect Dactyl: Trochee → Iamb → Iamb
	}

	return consort
}

type TemporalGrouper struct {
	WindowSize    time.Duration
	Buffer        []PulseEvent
	Groups        []*PulseTree
	PulseSequence *PulseSequence
	PendingPulses []PulseEvent
}

// AddPulse requires a minimum of three events to create a group
// This group will be analyzed for patterns
func (tg *TemporalGrouper) AddPulse(pulse PulseEvent) {
	// Add to current buffer
	tg.Buffer = append(tg.Buffer, pulse)

	// Init a PulseSequence if needed
	if tg.PulseSequence == nil {
		tg.PulseSequence = &PulseSequence{
			Events: []PulseEvent{},
		}
	}

	// Only add D1 pulses to the sequence for D2 pattern detection
	if pulse.Dimension == 1 {
		tg.PulseSequence.Events = append(tg.PulseSequence.Events, pulse)
		tg.PulseSequence.EndTime = pulse.StartTime

		// Detect D2 patterns only when there are exactly three pulses underneath
		if len(tg.PulseSequence.Events) >= 3 {
			consortPulses := tg.PulseSequence.DetectConsortPulses()
			for _, consortPulse := range consortPulses {
				tg.PendingPulses = append(tg.PendingPulses, consortPulse)
				slog.Debug("Adding consort")
			}

			// Sliding window: 3 D1 pulses,
			// chop off the one just processed that is
			// held in tg.PulseSequence.Events[0]
			if len(tg.PulseSequence.Events) > 3 {
				tg.PulseSequence.Events = tg.PulseSequence.Events[1:]
			}
		}

	}

	// Process any queued D2 pulses
	for len(tg.PendingPulses) > 0 {
		pending := tg.PendingPulses[0]
		tg.PendingPulses = tg.PendingPulses[1:]
		tg.Buffer = append(tg.Buffer, pending)
		slog.Debug("Processed pending pulse")
	}

	// Remove pulses outside the window
	limiter := time.Now().Add(-tg.WindowSize)
	tg.TrimBuffer(limiter)

	// Check if buffer has minimum pulses to form a group
	if len(tg.Buffer) >= 3 {
		// group := tg.createGroup()
		// tg.Groups = append(tg.Groups, group)
		tg.createGroupsByDimension()
	}
}

func (tg *TemporalGrouper) ProcessPendingPulses() {
	for len(tg.PendingPulses) > 0 {
		// This is the equivalent of an erlang head|tail match
		pending := tg.PendingPulses[0]
		tg.PendingPulses = tg.PendingPulses[1:]

		// Add directly to buffer without triggering more detection
		tg.Buffer = append(tg.Buffer, pending)
	}
}

func (tg *TemporalGrouper) createGroupsByDimension() {
	// Group pulses by dimension
	dimensionMap := make(map[int][]PulseEvent)
	for _, pulse := range tg.Buffer {
		dimensionMap[pulse.Dimension] = append(dimensionMap[pulse.Dimension], pulse)
	}

	// Create a group for each dimension that has enough pulses
	for dimension, pulses := range dimensionMap {
		if len(pulses) >= 3 {
			group := tg.CreateGroupForPulses(pulses, dimension)
			if group != nil {
				tg.Groups = append(tg.Groups, group)
			}
		}
	}
}

func (tg *TemporalGrouper) CreateGroupForPulses(pulses []PulseEvent, dimension int) *PulseTree {
	if len(pulses) == 0 {
		return nil
	}

	patterns := make([]PulsePattern, len(pulses))
	for i, p := range pulses {
		patterns[i] = p.Pattern
	}

	return &PulseTree{
		Dimension: dimension,
		Pulses:    patterns,
		StartTime: pulses[0].StartTime,
		Duration:  pulses[len(pulses)-1].StartTime.Sub(pulses[0].StartTime),
		Frequency: 1,
		Children:  nil,
	}
}

// TrimBuffer keeps the TG clean and allows for better memory management
func (tg *TemporalGrouper) TrimBuffer(limit time.Time) {
	// Find first pulse to KEEP (after limit)
	keepIndex := len(tg.Buffer) // Default: remove all
	for i, pulse := range tg.Buffer {
		if pulse.StartTime.After(limit) {
			keepIndex = i
			break
		}
	}

	// Keep only pulses inside the window
	if keepIndex < len(tg.Buffer) {
		copy(tg.Buffer, tg.Buffer[keepIndex:])
		tg.Buffer = tg.Buffer[:len(tg.Buffer)-keepIndex]
	} else {
		tg.Buffer = tg.Buffer[:0] // Clear all
	}

	// Clean groups
	kept := 0
	for _, group := range tg.Groups {
		if group.StartTime.After(limit) {
			tg.Groups[kept] = group
			kept++
		}
	}
	tg.Groups = tg.Groups[:kept]

	// Clean up PulseSequence
	if tg.PulseSequence != nil {
		keepIndex = len(tg.PulseSequence.Events)
		for i, pulse := range tg.PulseSequence.Events {
			if pulse.StartTime.After(limit) {
				keepIndex = i
				break
			}
		}

		if keepIndex < len(tg.PulseSequence.Events) {
			tg.PulseSequence.Events = tg.PulseSequence.Events[keepIndex:]
		} else {
			tg.PulseSequence.Events = tg.PulseSequence.Events[:0]
		}
	}
}
