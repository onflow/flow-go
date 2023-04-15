package trackers

import (
	"sync/atomic"
)

// EntityCompareFunc compares two entities by some ordering scheme.
// It returns true if a is ordered before b, and false otherwise.
// For all ordering schemes, nil orders before all possible values.
type EntityCompareFunc[E any] func(a, b *E) bool

// MonotonicEntityTracker tracks the largest value from a sequence of atomic writes.
// Only writes which are monotonically increasing w.r.t. the last write are persisted.
type MonotonicEntityTracker[E any] struct {
	cmp EntityCompareFunc[E]
	val *atomic.Pointer[E]
}

func NewNewestEntityTracker[E any](cmp EntityCompareFunc[E]) MonotonicEntityTracker[E] {
	return MonotonicEntityTracker[E]{
		cmp: cmp,
		val: new(atomic.Pointer[E]),
	}
}

// Track updates the tracked value if the input e is monotonically increasing w.r.t.
// the currently tracked value. Nil orders before all possible values for e:
//   - if the currently tracked value is nil, then e is always written
//   - if e is nil, it is never written
func (t *MonotonicEntityTracker[E]) Track(e *E) bool {
	// nil orders before all possible values, and is never written
	if e == nil {
		return false
	}
	for {
		// take a snapshot of the currently tracked value
		cur := t.Get()
		// verify that our update is monotonically increasing
		if cur != nil && !t.cmp(e, cur) {
			return false
		}
		// It is possible that another thread has written a new value since when
		// we read the atomic. In that case, the CAS operation will return false,
		// and we will try again in the next iteration.
		if t.val.CompareAndSwap(cur, e) {
			return true
		}
	}
}

// Get returns the currently tracked value.
func (t *MonotonicEntityTracker[E]) Get() *E {
	return t.val.Load()
}

type MonotonicUint64Tracker struct {
	val *atomic.Uint64
}

// Track updates the tracked value if the input is larger than the currently tracked value.
func (t *MonotonicUint64Tracker) Track(i uint64) bool {
	for {
		cur := t.Get()
		if i <= cur {
			return false
		}
		if t.val.CompareAndSwap(cur, i) {
			return true
		}
	}
}

// Get returns the currently tracked value.
func (t *MonotonicUint64Tracker) Get() uint64 {
	return t.val.Load()
}
