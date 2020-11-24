package topology

import (
	"math"
)

// FanoutFunc represents a function type that receiving total number of nodes
// in flow system, returns fanout of individual nodes.
type FanoutFunc func(size int) int

// LinearFanoutFunc guarantees full network connectivity in a deterministic way.
// Given system of `size` nodes, it returns `size+1/2`.
func LinearFanout(size int) int {
	fanout := math.Ceil(float64(size+1) / 2)
	return int(fanout)
}
