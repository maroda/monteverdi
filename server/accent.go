package monteverdi

import (
	"time"
)

// The Accent is the building block of this tool.
// What really should show up in the display is the accent,
// not the raw value.
type Accent struct {
	Timestamp int64       // Unix timestamp
	Intensity int         // raw, unweighted accent strength
	SourceID  string      // identifies the source
	DestLayer *Timeseries // identifies the output
}

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

func (a *Accent) ValToRune(val int64) rune {
	switch {
	case val < 1.0:
		return '▁'
	case val < 2.0:
		return '▂'
	case val < 3.0:
		return '▃'
	case val < 5.0:
		return '▄'
	case val < 8.0:
		return '▅'
	case val < 13.0:
		return '▆'
	case val < 21.0:
		return '▇'
	default:
		return '█'
	}
}

// AddSecond tallies each second as a counter
// then adds a rune to the slice indexed by second
func (a *Accent) AddSecond(val int64) {
	// This is the index of the rune, and also the current second
	a.DestLayer.Current = (a.DestLayer.Current + 1) % a.DestLayer.MaxSize

	// Translate the val parameter into a rune for display
	a.DestLayer.Runes[a.DestLayer.Current] = a.ValToRune(val)
}

// TODO: FIND THE RIGHT PLACE FOR THIS COUNTER
// This adds a second to the rolling timeseries using the metric value
// q.Network[ni].Accent[mv].AddSecond(metric)

// GetDisplay provides the string of runes for drawing
func (a *Accent) GetDisplay() []rune {
	display := make([]rune, a.DestLayer.MaxSize)
	for i := 0; i < a.DestLayer.MaxSize; i++ {
		idx := (a.DestLayer.Current + 1 + i) % a.DestLayer.MaxSize
		display[i] = a.DestLayer.Runes[idx]
	}
	return display
}
