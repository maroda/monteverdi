package monteverdi

import "time"

// The Accent is the building block of this tool.
// What really should show up in the display is the accent,
// not the raw value.

type EventMapping interface {
	TimestampString() string
}

type Accent struct {
	Timestamp time.Time
	Intensity int    // raw, unweighted accent strength
	SourceID  string // identifies the source
	DestLayer string // identifies the output
}

// NewAccent builds the metadata for the accent
// There is no boolean, the existence of an Accent is always true
func NewAccent(i int, s, d string) *Accent {
	return &Accent{
		Timestamp: time.Now(),
		Intensity: i,
		SourceID:  s,
		DestLayer: d,
	}
}

// TimestampString returns a compact string
// this can certainly be done in NewAccent,
// but just using this to run tests
func (a *Accent) TimestampString() string {
	return a.Timestamp.Format("20060102T150405")
}
