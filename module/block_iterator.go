package module

import "github.com/onflow/flow-go/model/flow"

// IterateJob defines the height range of blocks to iterate over
type IterateJob struct {
	Start uint64 // the start height of the range
	End   uint64 // the end height of the range
}

// BlockIterator is an interface for iterating over blocks
type BlockIterator interface {
	// Next returns the next block in the iterator
	// Note: a block will only be iterated once in a single iteration, however
	// if the iteration is interrupted (e.g. by a restart), the iterator can be
	// resumed from the last checkpoint, which might result in the same block being
	// iterated again.
	Next() (blockID flow.Identifier, hasNext bool, exception error)

	// Checkpoint saves the current state of the iterator
	// so that it can be resumed later
	Checkpoint() error
}

// SaveProgressFunc is a function that saves the progress of the iterator
type SaveProgressFunc func(uint64) error
