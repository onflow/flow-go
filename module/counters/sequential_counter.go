package counters

import "sync/atomic"

// SequentialCounter is a helper struct which implements a variation of a strict monotonous counter,
// where the value must be incremented by exactly one each time it is set.
// SequentialCounter is implemented using atomic operations and doesn't allow to set a value
// which is not exactly one larger than the already stored value. The counter is implemented
// solely with non-blocking atomic operations for concurrency safety.
type SequentialCounter struct {
	atomicCounter uint64
}

// NewSequentialCounter creates new counter with initial value
func NewSequentialCounter(initialValue uint64) SequentialCounter {
	return SequentialCounter{
		atomicCounter: initialValue,
	}
}

// Set updates value of counter if and only if it is exactly one larger than value which is already stored.
// Returns true if update was successful, otherwise it returns false.
func (c *SequentialCounter) Set(newValue uint64) bool {
	for {
		oldValue := c.Value()
		if newValue != oldValue+1 {
			return false
		}
		if atomic.CompareAndSwapUint64(&c.atomicCounter, oldValue, newValue) {
			return true
		}
	}
}

// Value returns value which is stored in atomic variable
func (c *SequentialCounter) Value() uint64 {
	return atomic.LoadUint64(&c.atomicCounter)
}
