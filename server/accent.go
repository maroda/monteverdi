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
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// The Accent is the building block of this tool.
type Accent struct {
	Timestamp int64  // Unix timestamp
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
	Metric                  string
	Events                  []Ictus
	StartTime               time.Time
	EndTime                 time.Time
	LastProcessedEventCount int
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
	Dimension int
	Pattern   PulsePattern
	StartTime time.Time // This is a Primary Key
	Duration  time.Duration
	Metric    []string
	Children  []time.Time // Primary Keys of children (D1 or greater)
	Parent    time.Time   // Primary Keys of a parent (D2 or greater)
}

type PulseEvents []PulseEvent

// FindChildren takes the StartTime of the Parent and returns its children
func (pe *PulseEvents) FindChildren(parentT time.Time) PulseEvents {
	var children PulseEvents
	for _, pulse := range *pe {
		if pulse.Parent.Equal(parentT) {
			children = append(children, pulse)
		}
	}
	return children
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
// recognizes two patterns that make up a Dimension 1 pulse:
// Iamb has no accent followed by an accent,
// Trochee has an accent followed by no accent.
func (is *IctusSequence) DetectPulsesWithConfig(config PulseConfig) []PulseEvent {
	slog.Debug("PULSE_DETECTION_INPUT",
		slog.Int("ictus_events", len(is.Events)),
		slog.String("metric", is.Metric))

	var pulses []PulseEvent

	// We need at least three events to process
	if len(is.Events) < 3 {
		return pulses
	}

	// Deduplication check
	lastProcessedCount := len(is.Events) - 1
	if lastProcessedCount == is.LastProcessedEventCount {
		return pulses
	}

	for i := 1; i < len(is.Events)-2; i++ {
		prev := is.Events[i-1]
		curr := is.Events[i]
		next := is.Events[i+1]

		// Iamb pattern: non-accent → accent transition
		if !prev.IsAccent && curr.IsAccent {
			slog.Debug("IAMB_DETECTED",
				slog.String("metric", is.Metric),
				slog.String("transition", fmt.Sprintf("%v->%v", prev.IsAccent, curr.IsAccent)))

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
			slog.Debug("TROCHEE_DETECTED",
				slog.String("metric", is.Metric),
				slog.String("transition", fmt.Sprintf("%v->%v", prev.IsAccent, curr.IsAccent)))

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

	// Track we've processed this sequence
	is.LastProcessedEventCount = lastProcessedCount

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
func (ps *PulseSequence) DetectConsortPulses(detectedKeys map[string]bool) []PulseEvent {
	// Need at least 3 pulses to detect patterns
	if ps == nil || ps.Events == nil || len(ps.Events) < 3 {
		return []PulseEvent{}
	}

	slog.Debug("CONSORT_DETECTION_START",
		slog.Int("events_count", len(ps.Events)),
		slog.String("first_pattern", fmt.Sprintf("%v", ps.Events[0].Pattern)),
		slog.String("second_pattern", fmt.Sprintf("%v", ps.Events[1].Pattern)),
		slog.String("third_pattern", fmt.Sprintf("%v", ps.Events[2].Pattern)))

	var consort []PulseEvent

	// Track what we've already detected to prevent duplicates
	detected := make(map[string]bool)

	for i := 0; i < len(ps.Events)-2; i++ {
		first := ps.Events[i]
		second := ps.Events[i+1]
		third := ps.Events[i+2]

		// Detect Amphibrach: Iamb → Trochee → Iamb
		// (non-accent→accent) → (accent→non-accent) → (non-accent→accent)
		if first.Pattern == Iamb && second.Pattern == Trochee && third.Pattern == Iamb {
			key := fmt.Sprintf("%d_%d_%d",
				first.StartTime.UnixNano(),
				second.StartTime.UnixNano(),
				third.StartTime.UnixNano())

			slog.Debug("AMPHIBRACH_DETECTED",
				slog.String("key", key),
				slog.Bool("already_detected", detected[key]))

			if detectedKeys[key] {
				continue
			}
			detected[key] = true

			slog.Debug("PULSE_TIMESTAMPS",
				slog.String("first_time", first.StartTime.Format("15:04:05.000")),
				slog.String("second_time", second.StartTime.Format("15:04:05.000")),
				slog.String("third_time", third.StartTime.Format("15:04:05.000")),
				slog.String("now", time.Now().Format("15:04:05.000")))

			newD2Pulse := PulseEvent{
				Dimension: 2,
				Pattern:   Amphibrach,
				StartTime: first.StartTime,
				Duration:  third.StartTime.Add(third.Duration).Sub(first.StartTime),
				Metric:    first.Metric,
				Children:  []time.Time{first.StartTime, second.StartTime, third.StartTime},
			}

			// Update this pulse as the parent for its Children
			first.Parent = newD2Pulse.StartTime
			second.Parent = newD2Pulse.StartTime
			third.Parent = newD2Pulse.StartTime

			// Add the pulse to the consort
			consort = append(consort, newD2Pulse)

			slog.Debug("NEW CONSORT PATTERN", slog.Any("event", newD2Pulse))
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
	DetectedKeys  map[string]bool
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

		// Detect D2 patterns when there are exactly three pulses
		if len(tg.PulseSequence.Events) == 3 {
			// Get the last 3 events only
			recentEvents := tg.PulseSequence.Events[len(tg.PulseSequence.Events)-3:]

			// Sort by timestamp before detecting patterns!!!
			sort.Slice(recentEvents, func(i, j int) bool {
				return recentEvents[i].StartTime.Before(recentEvents[j].StartTime)
			})

			// Create temporary sequence with chronologically ordered triplet
			tempSeq := &PulseSequence{
				Metric: tg.PulseSequence.Metric,
				Events: recentEvents,
			}

			slog.Debug("PATTERN_DETECTION_RATE",
				slog.String("pattern_sequence", fmt.Sprintf("%v->%v->%v",
					recentEvents[0].Pattern, recentEvents[1].Pattern, recentEvents[2].Pattern)),
				slog.Int("buffer_size", len(tg.Buffer)))

			consortPulses := tempSeq.DetectConsortPulses(tg.DetectedKeys)
			for _, consortPulse := range consortPulses {
				tg.PendingPulses = append(tg.PendingPulses, consortPulse)
				slog.Debug("Adding consort")
			}

			d1Count, d2Count := 0, 0
			for _, p := range tg.Buffer {
				if p.Dimension == 1 {
					d1Count++
				} else {
					d2Count++
				}
			}
			slog.Debug("DIMENSION_COUNTS", slog.Int("d1", d1Count), slog.Int("d2", d2Count))

			// Sliding window: reset to empty after processing
			slog.Debug("SEQUENCE_BEFORE_WINDOW", slog.Any("events", tg.PulseSequence.Events))
			tg.PulseSequence.Events = []PulseEvent{} // Clear the sequence
			slog.Debug("SEQUENCE_AFTER_WINDOW", slog.Any("events", tg.PulseSequence.Events))
		}
	}

	// Process any queued D2 pulses
	for len(tg.PendingPulses) > 0 {
		pending := tg.PendingPulses[0]
		tg.PendingPulses = tg.PendingPulses[1:]
		tg.Buffer = append(tg.Buffer, pending)

		// LOG: When amphibrach is processed from pending
		if pending.Dimension == 2 {
			slog.Debug("AMPHIBRACH_PENDING_PROCESSED",
				slog.Float64("age_seconds", time.Since(pending.StartTime).Seconds()))
		}
	}

	// Remove pulses outside the window
	// NB: /tg.WindowSize is a data retention and analysis window
	// and the following is a memory management window
	// This number directly affects how long pulses can be displayed
	removalWindow := 600 * time.Second
	limiter := time.Now().Add(-removalWindow)

	tg.TrimBuffer(limiter)

	// LOG: After trimming
	amphibrachCountAfter := 0
	for _, p := range tg.Buffer {
		if p.Dimension == 2 {
			amphibrachCountAfter++
		}
	}

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
// TODO: Amphibrach trimming is not working
func (tg *TemporalGrouper) TrimBuffer(limit time.Time) {
	initialCount := len(tg.Buffer)

	// Find first pulse to KEEP (after limit)
	keepIndex := len(tg.Buffer) // Default: remove all
	// tg.Buffer is a []PulseEvent
	for i, pulse := range tg.Buffer {
		// the pulse.StartTime happens after the limit in the past, it's a keeper
		if pulse.StartTime.After(limit) {
			keepIndex = i
			break
		}
	}

	slog.Debug("TRIM_DEBUG",
		slog.Int("initial_count", initialCount),
		slog.Int("keep_index", keepIndex),
		slog.Int("removing", keepIndex),
		slog.String("limit", limit.Format("15:04:05.000")))

	// Keep only pulses inside the window
	if keepIndex < len(tg.Buffer) {
		tg.Buffer = tg.Buffer[keepIndex:] // Keep everything past keepIndex
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
