package plugin

/*

	The Adapter sits aside /monteverdi/
	Contains core interfaces for Plugin

*/

import (
	"time"

	Mt "github.com/maroda/monteverdi/types"
)

// MetricTransformer is an example of an interface to use for a Plugin
// The Transform wrapper returns the int64 to compare with Maxval
// An ID or Type for the transformer that is descriptive of what it does
// The amount of hysteresis needed for the calculation,
// for instance rates need 1, moving average 4,
// derivative (acceleration) 2, simple threshold check 0.
type MetricTransformer interface {
	Transform(metric string, current int64, historical []int64, timestamp time.Time) (int64, error)
	HysteresisReq() int // Required measurements in the past needed for calculation
	Type() string       // Unique ID for the transformer
}

// OutputAdapter can be used to define a place for the data to go,
// pulse-by-pulse or in batches if supported by the output type.
type OutputAdapter interface {
	WritePulse(pulse *Mt.PulseEvent) error                     // Write singleton pulse data
	WriteBatch(pulses []*Mt.PulseEvent) error                  // Write batches of pulses
	QueryRange(start, end time.Time) ([]*Mt.PulseEvent, error) // Time range query tool
	Flush() error                                              // Flush any buffered data
	Close() error                                              // Close the adapter and release resources
	Type() string                                              // ID for output
}
