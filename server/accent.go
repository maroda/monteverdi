package monteverdi

import (
	"time"
)

// The Accent is the building block of this tool.
// What really should show up in the display is the accent,
// not the raw value.
type Accent struct {
	Timestamp int64  // Unix timestamp
	Intensity int    // raw, unweighted accent strength
	SourceID  string // identifies the source
	DestLayer string // identifies the output
}

// NewAccent builds the metadata for the accent
// There is no boolean, the existence of an Accent is always true
func NewAccent(i int, s, d string) *Accent {
	// does this need to know the maxval?
	return &Accent{
		Timestamp: time.Now().UnixNano(),
		Intensity: i,
		SourceID:  s,
		DestLayer: d,
	}
}
