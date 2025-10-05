package plugin

/*

	The Adapter sits aside /monteverdi/
	Contains core interfaces for Plugin

*/

import (
	"time"
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

type Registry struct {
	transformers map[string]MetricTransformer
}
