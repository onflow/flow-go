package module

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// IterateRange defines the range of blocks to iterate over
// the range could be either view based range or height based range.
// when specifying the range, the start and end are inclusive, and the end must be greater than or
// equal to the start
type IterateRange struct {
	Start uint64 // the start of the range
	End   uint64 // the end of the range
}

// IterateRangeCreator is an interface for creating iterate jobs
type IteratorJobCreator interface {
	// CreateJob takes a progress reader which is used to read the progress of the iterator
	// and returns an iterate job that specifies the range of blocks to iterate over
	CreateJob(IterateProgressReader) (IterateRange, error)
}

// IterateProgressReader reads the progress of the iterator, useful for resuming the iteration
// after restart
type IterateProgressReader interface {
	// LoadState reads the next block to iterate
	// caller must ensure the reader is created by the IterateProgressInitializer,
	// otherwise LoadState would return exception.
	LoadState() (uint64, error)
}

// IterateProgressWriter saves the progress of the iterator
type IterateProgressWriter interface {
	// SaveState persists the next block to be iterated
	SaveState(uint64) error
}

// IterateProgressInitializer is an interface for initializing the progress of the iterator
// a initializer must be used to ensures the initial next block to be iterated is saved in
// storage before creating the block iterator
type IterateProgressInitializer interface {
	Init() (IterateProgressReader, IterateProgressWriter, error)
}

// BlockIterator is an interface for iterating over blocks
type BlockIterator interface {
	// Next returns the next block in the iterator
	// Note: this method is not concurrent-safe
	// Note: a block will only be iterated once in a single iteration, however
	// if the iteration is interrupted (e.g. by a restart), the iterator can be
	// resumed from the last checkpoint, which might result in the same block being
	// iterated again.
	// TODO: once upgraded to go 1.23, consider using the Range iterator
	//   Range() iter.Seq2[flow.Identifier, error]
	//   so that the iterator can be used in a for loop:
	//   for blockID, err := range heightIterator.Range()
	Next() (blockID flow.Identifier, hasNext bool, exception error)

	// Checkpoint saves the current state of the iterator
	// so that it can be resumed later
	// when Checkpoint is called, if SaveStateFunc is called with block A,
	// then after restart, the iterator will resume from A.
	// make sure to call this after all the jobs for processing the block IDs returned by
	// Next() are completed.
	Checkpoint() error
}

// IteratorCreator is an interface for creating block iterators
type IteratorCreator interface {
	// CreateIterator takes iterate job which specifies the range of blocks to iterate over
	// and a progress writer which is used to save the progress of the iterator,
	// and returns a block iterator that can be used to iterate over the blocks
	// Note: it's up to the implementation to decide how often the progress is saved,
	// it is wise to consider the trade-off between the performance and the progress saving,
	// if the progress is saved too often, it might impact the iteration performance, however,
	// if the progress is only saved at the end of the iteration, then if the iteration
	// was interrupted, then the iterator will start from the beginning of the range again,
	// which means some blocks might be iterated multiple times.
	CreateIterator(IterateRange, IterateProgressWriter) (BlockIterator, error)
}

type IteratorFactory struct {
	progressReader IterateProgressReader
	progressWriter IterateProgressWriter
	creator        IteratorCreator
	jobCreator     IteratorJobCreator
}

func NewIteratorFactory(
	initializer IterateProgressInitializer,
	creator IteratorCreator,
	jobCreator IteratorJobCreator,
) (*IteratorFactory, error) {
	progressReader, progressWriter, err := initializer.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize progress: %w", err)
	}

	return &IteratorFactory{
		progressReader: progressReader,
		progressWriter: progressWriter,
		creator:        creator,
		jobCreator:     jobCreator,
	}, nil
}

func (f *IteratorFactory) Create() (BlockIterator, error) {
	job, err := f.jobCreator.CreateJob(f.progressReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create job for block iteration: %w", err)
	}

	iterator, err := f.creator.CreateIterator(job, f.progressWriter)
	if err != nil {
		return nil, fmt.Errorf("failed to create block iterator: %w", err)
	}

	return iterator, nil
}
