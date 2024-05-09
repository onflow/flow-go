package counters

import "sync/atomic"

// StrictMonotonousCounter is a helper struct which implements a strict monotonous counter.
// StrictMonotonousCounter is implemented using atomic operations and doesn't allow to set a value
// which is lower or equal to the already stored one. The counter is implemented
// solely with  non-blocking atomic operations for concurrency safety.
type StrictMonotonousCounter struct {
	atomicCounter uint64
}

// NewMonotonousCounter creates new counter with initial value
func NewMonotonousCounter(initialValue uint64) StrictMonotonousCounter {
	return StrictMonotonousCounter{
		atomicCounter: initialValue,
	}
}

// Set updates value of counter if and only if it's strictly larger than value which is already stored.
// Returns true if update was successful or false if stored value is larger.
func (c *StrictMonotonousCounter) Set(newValue uint64) bool {
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
func (c *StrictMonotonousCounter) Value() uint64 {
	return atomic.LoadUint64(&c.atomicCounter)
}
