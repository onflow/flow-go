package height_based

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type HeightIterator struct {
	// dependencies
	headers  storage.Headers
	progress module.IterateProgressWriter // for saving the next height to be iterated for resuming the iteration

	// config
	endHeight uint64

	// state
	nextHeight uint64
}

var _ module.BlockIterator = (*HeightIterator)(nil)

// caller must ensure that both job.Start and job.End are finalized height
func NewHeightIterator(
	headers storage.Headers,
	progress module.IterateProgressWriter,
	job module.IterateRange,
) (module.BlockIterator, error) {
	return &HeightIterator{
		headers:    headers,
		progress:   progress,
		endHeight:  job.End,
		nextHeight: job.Start,
	}, nil
}

// Next returns the next block ID in the iteration
// it iterates from lower height to higher height.
// when iterating a height, it iterates over all sibling blocks at that height
func (b *HeightIterator) Next() (flow.Identifier, bool, error) {
	if b.nextHeight > b.endHeight {
		return flow.ZeroID, false, nil
	}

	// TODO: use storage operation instead to avoid hitting cache
	next, err := b.headers.BlockIDByHeight(b.nextHeight)
	if err != nil {
		return flow.ZeroID, false, fmt.Errorf("failed to fetch block at height %v: %w", b.nextHeight, err)
	}

	b.nextHeight++

	return next, true, nil
}

// Checkpoint saves the iteration progress to storage
func (b *HeightIterator) Checkpoint() error {
	err := b.progress.SaveState(b.nextHeight)
	if err != nil {
		return fmt.Errorf("failed to save progress at view %v: %w", b.nextHeight, err)
	}
	return nil
}
