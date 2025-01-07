package module

import "github.com/onflow/flow-go/model/flow"

// IterateJob defines the range of blocks to iterate over
// the range could be either view based range or height based range.
// when specifying the range, the start and end are inclusive, and the end must be greater than or
// equal to the start
type IterateJob struct {
	Start uint64 // the start of the range
	End   uint64 // the end of the range
}

// IterateJobCreator is an interface for creating iterate jobs
type IteratorJobCreator interface {
	// CreateJob takes a progress reader which is used to read the progress of the iterator
	// and returns an iterate job that specifies the range of blocks to iterate over
	CreateJob(IterateProgressReader) (IterateJob, error)
}

type IterateProgressReader interface {
	// ReadNext reads the next to iterate
	ReadNext() (uint64, error)
}

type IterateProgressWriter interface {
	// SaveNext persists the next block to be iterated
	SaveNext(uint64) error
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

// IteratorCreator is an interface for creating block iterators
type IteratorCreator interface {
	// CreateIterator takes iterate job which specifies the range of blocks to iterate over
	// and a progress writer which is used to save the progress of the iterator,
	// and returns a block iterator that can be used to iterate over the blocks
	// if the end of the
	CreateIterator(IterateJob, IterateProgressWriter) (BlockIterator, error)
}
