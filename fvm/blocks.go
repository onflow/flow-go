package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// TODO figure out errors
type Blocks interface {
	// returns current block header
	Current() (runtime.Block, error)
	// ByHeight returns the block at the given height in the chain ending in `header` (or finalized
	// if `header` is nil). This enables querying un-finalized blocks by height with respect to the
	// chain defined by the block we are executing. It returns a runtime block,
	// a boolean which is set if block is found and an error if any fatal error happens
	ByHeight(height uint64) (runtime.Block, bool, error)
}

// BlocksFinder finds blocks and return block headers
type BlocksFinder struct {
	minHeightAvailable uint64 // inclusive
	maxHeightAvailable uint64 // inclusive
	header             *flow.Header
	storage            storage.Headers
}

// NewBlockFinder constructs a new block finder
func NewBlockFinder(header *flow.Header, storage storage.Headers, minHeightAvailable uint64, maxHeightAvailable uint64) Blocks {
	return &BlocksFinder{
		header:             header,
		minHeightAvailable: minHeightAvailable,
		maxHeightAvailable: maxHeightAvailable,
		storage:            storage,
	}
}

// TODO we might evaluate the header first and return error if not exist
func (b *BlocksFinder) Current() (runtime.Block, error) {
	return RuntimeBlockFromFlowHeader(b.header), nil
}

// ByHeightFrom returns the block header by height.
func (b *BlocksFinder) ByHeight(height uint64) (runtime.Block, bool, error) {
	// don't return any block from the future or
	// from before root block or starting block of the spork.
	if height > b.maxHeightAvailable || height < b.minHeightAvailable {
		msg := fmt.Sprintf("requested height (%d) is not in the range(%d, %d)", height, b.minHeightAvailable, b.maxHeightAvailable)
		err := errors.NewValueErrorf(fmt.Sprint(height), msg)
		return runtime.Block{}, false, fmt.Errorf("cannot retrieve block parent: %w", err)
	}

	// this is provided for compability reasons
	// we might remove it (TODO)
	if b.header == nil {
		byHeight, err := b.storage.ByHeight(height)
		if err != nil {
			return runtime.Block{}, false, err
		}
		return RuntimeBlockFromFlowHeader(byHeight), true, nil
	}

	// if the height is for the header just return the header
	if height == b.header.Height {
		return RuntimeBlockFromFlowHeader(b.header), true, nil
	}

	id := b.header.ParentID

	// travel chain back
	for {
		// recent block should be in cache so this is supposed to be fast
		parent, err := b.storage.ByBlockID(id)
		if err != nil {
			failure := errors.NewBlockFinderFailure(err)
			return runtime.Block{}, false, fmt.Errorf("cannot retrieve block parent: %w", failure)
		}
		if parent.Height == height {
			return RuntimeBlockFromFlowHeader(parent), true, nil
		}

		_, err = b.storage.ByHeight(parent.Height)
		// if height isn't finalized, move to parent
		if err != nil && errors.Is(err, storage.ErrNotFound) {
			id = parent.ParentID
			continue
		}
		// any other error bubbles up
		if err != nil {
			failure := errors.NewBlockFinderFailure(err)
			return runtime.Block{}, false, fmt.Errorf("cannot retrieve block parent: %w", failure)
		}
		//if parent is finalized block, we can just use finalized chain
		// to get desired height
		bh, err := b.storage.ByHeight(height)

		if errors.Is(err, storage.ErrNotFound) {
			return runtime.Block{}, false, nil
		} else if err != nil {
			failure := errors.NewBlockFinderFailure(err)
			return runtime.Block{}, false, fmt.Errorf("getting block at height failed for height %v: %w", height, failure)
		}
		return RuntimeBlockFromFlowHeader(bh), true, nil
	}
}

func RuntimeBlockFromFlowHeader(header *flow.Header) runtime.Block {
	return runtime.Block{
		Height:    header.Height,
		View:      header.View,
		Hash:      runtime.BlockHash(header.ID()),
		Timestamp: header.Timestamp.UnixNano(),
	}
}
