package monteverdi

import (
	"time"
)

// The Accent is the building block of this tool.
// What really should show up in the display is the accent,
// not the raw value.

// Conductor is a set of methods on the Accent and its configuration
type Conductor interface {
	init()
}

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

// Init will run the first time an accent of this SourceID is registered
func (a *Accent) init() {
	panic("not implemented")
	// read json
	// a.ReadConfig()
	// parse config
	// validate config
	// create Endpoint from config
	// add Endpoint to Endpoints
	// possibly create QNet
}
