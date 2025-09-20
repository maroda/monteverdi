package monteverdi

import (
	"time"
)

// The Accent is the building block of this tool.
type Accent struct {
	Timestamp int64       // Unix timestamp TODO: this should be time.Time
	Intensity int         // raw, unweighted accent strength
	SourceID  string      // identifies the source
	DestLayer *Timeseries // identifies the output
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
		DestLayer: &Timeseries{
			Runes:   make([]rune, 60),
			MaxSize: 60,
		},
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
)

type PulseEvent struct {
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
				Pattern:   Iamb,
				StartTime: curr.Timestamp,
				Duration:  next.Timestamp.Sub(curr.Timestamp),
				Metric:    []string{is.Metric},
			})
		}

		// Detect Trochee: accent → non-accent
		if curr.IsAccent && !next.IsAccent {
			pulses = append(pulses, PulseEvent{
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
	Dimension int            // 0=individual Iamb / Trochee, 1=phrases, 2=periods
	Pulses    []PulsePattern // The constituent pulses
	OGEvents  []PulseEvent   // Preserve source event data
	StartTime time.Time
	Duration  time.Duration
	Frequency int             // How often this grouping occurs
	Children  []*PulseTree    // Lower-level patterns that comprise this one
	VizData   []PulseVizPoint // Generic visualization descriptor
}

type PulseVizPoint struct {
	Position  int          // 0-59 on the timeline
	Pattern   PulsePattern // Iamb or Trochee
	IsAccent  bool         // Is an accent
	Duration  time.Duration
	StartTime time.Time
	Extends   bool // pattern extends beyond display
}

type TemporalGrouper struct {
	WindowSize time.Duration
	Buffer     []PulseEvent
	Groups     []*PulseTree
}

func (tg *TemporalGrouper) AddPulse(pulse PulseEvent) {
	// Add to current buffer
	tg.Buffer = append(tg.Buffer, pulse)

	// Remove pulses outside the window
	limiter := time.Now().Add(-tg.WindowSize)
	tg.TrimBuffer(limiter)

	// Check if buffer has minimum pulses to form a group
	if len(tg.Buffer) >= 3 {
		group := tg.createGroup()
		tg.Groups = append(tg.Groups, group)
	}
}

func (tg *TemporalGrouper) createGroup() *PulseTree {
	pulses := make([]PulsePattern, len(tg.Buffer))
	for i, p := range tg.Buffer {
		pulses[i] = p.Pattern
	}

	return &PulseTree{
		Dimension: 1, // Phrase level, could be dynamic?
		Pulses:    pulses,
		StartTime: tg.Buffer[0].StartTime,
		Duration:  tg.Buffer[len(tg.Buffer)-1].StartTime.Sub(tg.Buffer[0].StartTime),
		Frequency: 1, // Track this over time...
		Children:  nil,
	}
}

func (tg *TemporalGrouper) TrimBuffer(limit time.Time) {
	// Find the first pattern that is still inside the window
	keepIndex := 0
	for i, pulse := range tg.Buffer {
		if pulse.StartTime.After(limit) {
			keepIndex = i
			break
		}
		keepIndex = len(tg.Buffer) // If no pulses are after limit, remove all
	}

	// Keep only pulses inside the window
	if keepIndex < len(tg.Buffer) {
		tg.Buffer = tg.Buffer[keepIndex:]
	} else {
		tg.Buffer = tg.Buffer[:0] // Clear the buffer
	}
}
