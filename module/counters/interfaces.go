package counters

// Reader is an interface for reading the value of a counter.
type Reader interface {
	// Value returns the current value of the counter.
	Value() uint64
}
