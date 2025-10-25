package types

/*

	These are the "immutable" core types of Monteverdi,
	provided for cross-package use (e.g. Plugins) and testing.

	There are no functions defined here.
	Struct constructors are housed in their own packages.
	Methods taking these types should create local aliases,
	for example: type PulseEvents []Mt.PulseEvent

*/

import "time"

// The Accent is the building block of this tool.
// When the Maxval is triggered, the Accent starts.
// When the metric drops below Maxval, the Accent ends.
type Accent struct {
	Intensity int    // unweighted accent strength
	SourceID  string // identifies the source
	Timestamp int64  // Unix nanosecond timestamp
}

// Timeseries is a generic fixed TimeSeries DB
type Timeseries struct {
	Runes   []rune
	MaxSize int
	Current int
}

// The Ictus is event in time.
// It is either the trigger of an accent period,
// or the beginning of a no-accent period.
type Ictus struct {
	IsAccent  bool
	Value     int64
	Timestamp time.Time
	Duration  time.Duration
}

// These trigrams are read top-down, representing an accent/non-accent sequence
// These are used in the UI to illuminate the pattern when it happens.
// Mostly these are unused constants, but here for reference.
const (
	accent     = "⚊" // U+268B is on (yang)
	nonAccent  = "⚋" // U+268A is off (yin)
	iamb       = "⚍" // U+268D is off, on (lesser yin)
	trochee    = "⚎" // U+268E is on, off (lesser yang)
	amphibrach = "☵" // U+2635 is off, on, off (water)
	anapest    = "☳" // U+2633 is off, off, on (thunder)
	dactyl     = "☶" // U+2636 is on, off, off (mountain)
)

// PulsePattern is the seed of Meyer pattern matching
// But why pulse?
// Meyer explicitly points out the difference between rhythm and pulse.
// Accents define the pulse of something, which are separate from its regular rhythms.
// For example, the expected rhythm of an operational system can include health checks,
// garbage collection, predictable load, etc. But the pulse represents a deeper, more
// fundamental force as the system interacts with the real-world, as its rhythms continue.
type PulsePattern int

// These are the actual pattern data types for each pulse
const (
	Iamb       PulsePattern = iota // Iamb: non-accent → accent
	Trochee                        // Trochee: accent → non-accent
	Amphibrach                     // Amphibrach: non-accent → accent → non-accent
	Anapest                        // Anapest: non-accent → non-accent → accent (not yet implemented)
	Dactyl                         // Dactyl: accent → non-accent → non-accent (not yet implemented)
)

// PulseEvent is the pulse metadata
type PulseEvent struct {
	Dimension int
	Metric    []string
	Pattern   PulsePattern
	Parent    time.Time   // Primary Keys of a parent (D2 or greater)
	Children  []time.Time // Primary Keys of children (D1 or greater)
	StartTime time.Time   // This is a Primary Key
	Duration  time.Duration
}

// PulseTree is a data structure to order pulses between dimensions.
type PulseTree struct {
	Dimension int
	Frequency int             // How often this grouping occurs
	Pulses    []PulsePattern  // The constituent pulses
	OGEvents  []PulseEvent    // Preserve source event data
	Children  []*PulseTree    // Lower-level patterns that comprise this one
	VizData   []PulseVizPoint // Generic visualization descriptor
	StartTime time.Time
	Duration  time.Duration
}

// PulseVizPoint is metadata for drawing the pulse in the UI
type PulseVizPoint struct {
	Position  int          // position on the timeline
	IsAccent  bool         // Is it an accent?
	Extends   bool         // pattern extends beyond display (for pulse view)
	Pattern   PulsePattern // pattern to show
	StartTime time.Time
	Duration  time.Duration
}

// PulseConfig is the Dimension 1 period configuration,
// typically all are set to the middle (.5) to detect the pattern.
// This is mostly here for future configuration UI features
type PulseConfig struct {
	IambStartPeriod    float64 // 0.0 = start of non-accent, 0.5 = middle, 1.0 = end
	IambEndPeriod      float64 // 0.0 = start of accent, 0.5 = middle, 1.0 = end
	TrocheeStartPeriod float64 // For accent period
	TrocheeEndPeriod   float64 // For non-accent period
}
