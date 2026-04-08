package limiters

import (
	"errors"

	"go.uber.org/atomic"
)

// ConcurrencyLimiter is a limiter that allows a maximum number of concurrent operations to be executed.
//
// Safe for concurrent use.
type ConcurrencyLimiter struct {
	maxConcurrent   uint32
	totalConcurrent *atomic.Uint32
}

// NewConcurrencyLimiter creates a new ConcurrencyLimiter with the given maximum number of concurrent operations.
func NewConcurrencyLimiter(maxConcurrent uint32) (*ConcurrencyLimiter, error) {
	if maxConcurrent == 0 {
		return nil, errors.New("maxConcurrent must be greater than 0")
	}

	return &ConcurrencyLimiter{
		maxConcurrent:   maxConcurrent,
		totalConcurrent: atomic.NewUint32(0),
	}, nil
}

// Acquire atomically increments the concurrency counter if it is below the configured limit.
// Returns true if the slot was acquired, false if the limit was reached.
// The caller MUST call [Release] exactly once after a successful Acquire.
func (h *ConcurrencyLimiter) Acquire() bool {
	for {
		current := h.totalConcurrent.Load()
		if current >= h.maxConcurrent {
			return false
		}
		if h.totalConcurrent.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

// Release decrements the concurrency counter, freeing a slot previously obtained via [Acquire].
// Must be called exactly once for every successful [Acquire] call.
func (h *ConcurrencyLimiter) Release() {
	h.totalConcurrent.Sub(1)
}

// Allow executes fn if the number of concurrent operations is below the configured limit.
// Returns true if fn was executed, false if the limit was reached and fn was not called.
// The concurrency counter is decremented when fn returns, including on panic.
func (h *ConcurrencyLimiter) Allow(fn func()) bool {
	if !h.Acquire() {
		return false
	}
	// decrement within a defer to support usecases where panics are handled gracefully by the caller
	defer h.Release()

	fn()
	return true
}
