package counters

import "sync/atomic"

// StrictMonotonicCounter is a helper struct which implements a strict monotonic counter.
// StrictMonotonicCounter is implemented using atomic operations and doesn't allow to set a value
// which is lower or equal to the already stored one. The counter is implemented
// solely with  non-blocking atomic operations for concurrency safety.
type StrictMonotonicCounter struct {
	atomicCounter uint64
}

// NewMonotonicCounter creates new counter with initial value
func NewMonotonicCounter(initialValue uint64) StrictMonotonicCounter {
	return StrictMonotonicCounter{
		atomicCounter: initialValue,
	}
}

// Set updates value of counter if and only if it's strictly larger than value which is already stored.
// Returns true if update was successful or false if stored value is larger.
func (c *StrictMonotonicCounter) Set(newValue uint64) bool {
	for {
		oldValue := c.Value()
		if newValue <= oldValue {
			return false
		}
		if atomic.CompareAndSwapUint64(&c.atomicCounter, oldValue, newValue) {
			return true
		}
	}
}

// Value returns value which is stored in atomic variable
func (c *StrictMonotonicCounter) Value() uint64 {
	return atomic.LoadUint64(&c.atomicCounter)
}

// Increment atomically increments counter and returns updated value
func (c *StrictMonotonicCounter) Increment() uint64 {
	for {
		oldValue := c.Value()
		newValue := oldValue + 1
		if atomic.CompareAndSwapUint64(&c.atomicCounter, oldValue, newValue) {
			return newValue
		}
	}
}
