package topology

// FanoutFunc represents a function type that receiving total number of nodes
// in flow system, returns fanout of individual nodes.
type FanoutFunc func(size uint) uint

// LinearFanoutFunc guarantees full network connectivity in a deterministic way.
// Given system of `size` nodes, it returns `size+1/2`.
var LinearFanoutFunc FanoutFunc = func(size uint) uint {
	return uint(size+1) / 2
}
