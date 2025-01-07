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
	// when Checkpoint is called, if SaveNextFunc is called with block A,
	// then after restart, the iterator will resume from A.
	Checkpoint() error
}

// SaveNextFunc is a function that saves the next block to be iterated
// the uint64 argument is the height or view of the block, depending on the iterator
type SaveNextFunc func(uint64) error
