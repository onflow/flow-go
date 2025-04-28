package operation

import (
	"errors"

	"github.com/onflow/flow-go/storage"
)

// multiIterator represents a logical concatenation of multiple iterators
// in sequence.  multiIterator iterates items in the first iterator, and then
// iterates items in the second iterator, etc.
// Invariants:
// - len(iterators) is > 0
// - cur is [0, len(iterator)-1]
type multiIterator struct {
	iterators []storage.Iterator
	cur       int // index of current iterator
}

var _ storage.Iterator = (*multiIterator)(nil)

// NewMultiIterator returns an Iterator that is a logical concatenation of
// multiple iterators in the provided sequence.  The returned iterator
// iterates items in the first iterator, and then iterates items in
// the second iterator, etc.
func NewMultiIterator(iterators ...storage.Iterator) (storage.Iterator, error) {
	if len(iterators) == 0 {
		return nil, errors.New("failed to create multiIterator: need at least one iterator")
	}
	if len(iterators) == 1 {
		return iterators[0], nil
	}
	return &multiIterator{iterators: iterators}, nil
}

// First seeks to the smallest key greater than or equal to the given key.
func (mi *multiIterator) First() bool {
	for i, iterator := range mi.iterators {
		mi.cur = i

		valid := iterator.First()
		if valid {
			return true
		}
	}

	return false
}

// Valid returns whether the iterator is positioned at a valid key-value pair.
func (mi *multiIterator) Valid() bool {
	return mi.iterators[mi.cur].Valid()
}

// Next advances the iterator to the next key-value pair.
func (mi *multiIterator) Next() {
	// Move to next item in the current iterator.
	mi.iterators[mi.cur].Next()

	// Return if next item is valid or end of last iterator is reached.
	if mi.Valid() || mi.isLast() {
		return
	}

	// Move to next iterator.
	mi.cur++

	for i := mi.cur; i < len(mi.iterators); i++ {
		mi.cur = i

		valid := mi.iterators[mi.cur].First()
		if valid {
			return
		}
	}
}

// IterItem returns the current key-value pair, or nil if done.
func (mi *multiIterator) IterItem() storage.IterItem {
	return mi.iterators[mi.cur].IterItem()
}

// Close closes the iterator.
// Iterator must be closed, otherwise it causes memory leaks.
// No errors are expected during normal operation.
func (mi *multiIterator) Close() error {
	var errs []error
	for _, iterator := range mi.iterators {
		err := iterator.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (mi *multiIterator) isLast() bool {
	return mi.cur == len(mi.iterators)-1
}
