package indexer

import (
	"fmt"
	"sync/atomic"
)

var (
	ErrIndexValue    = fmt.Errorf("invalid index value")
	ErrIndexBoundary = fmt.Errorf("the value is not within the indexed interval")
)

// SequentialIndexRange contains an interval bounded by two whole numbers, and it can only be increased
// by incrementing the last value by exactly one, or if the same value is provide we ignore it to make the
// mutation idempotent. This can be useful when we want to index height range.
// SequentialIndexRange makes all the mutations atomic, so it can be considered concurrent-safe.
type SequentialIndexRange struct {
	first uint64 // we never allow changing first so no need to be atomic
	last  *atomic.Uint64
}

// NewSequentialIndexRange creates new index range with the provided first and last values.
func NewSequentialIndexRange(first uint64, last uint64) *SequentialIndexRange {
	var lastAtomic *atomic.Uint64
	lastAtomic.Store(last)

	return &SequentialIndexRange{
		first: first,
		last:  lastAtomic,
	}
}

// First return the first index in the range.
func (i *SequentialIndexRange) First() uint64 {
	return i.first
}

// Last return the last index in the range.
func (i *SequentialIndexRange) Last() uint64 {
	return i.last.Load()
}

// Increase the index range with the provided value. The value should be exactly one bigger than the current value
// or can be same to make the action idempotent.
// Expected errors:
// - ErrIndexValue if the value is not incremented by one or equal to the last value
func (i *SequentialIndexRange) Increase(new uint64) error {
	if ok, err := i.CanIncrease(new); !ok {
		return err
	}

	last := i.Last()
	for {
		if i.last.CompareAndSwap(last, new) {
			return nil
		}
	}
}

// CanIncrease the index range with the provided value. The value should be exactly one bigger than the current value
// or can be same to make the action idempotent.
// Expected errors:
// - ErrIndexValue if the value is not incremented by one or equal to the last value
func (i *SequentialIndexRange) CanIncrease(new uint64) (bool, error) {
	last := i.Last()
	diff := new - last
	if diff < 0 || diff > 1 {
		return false, fmt.Errorf("value %d should be equal or incremented by one from the last indexed value: %d: %w", new, last, ErrIndexValue)
	}

	return true, nil
}

// Contained checks if the value is contained within the interval. The boundary check is inclusive.
// Expected errors:
// - ErrIndexBoundary if the value is out of indexed boundary
func (i *SequentialIndexRange) Contained(val uint64) (bool, error) {
	first := i.First()
	last := i.Last()

	if val < first || val > last {
		return false, fmt.Errorf("value %d is out of boundary [%d - %d]: %w", val, i.first, i.last, ErrIndexBoundary)
	}

	return true, nil
}
