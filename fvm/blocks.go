package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

type Blocks interface {
	// ByHeight returns the block at the given height in the chain ending in `header` (or finalized
	// if `header` is nil). This enables querying un-finalized blocks by height with respect to the
	// chain defined by the block we are executing.
	ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error)
}

// BlocksFinder finds blocks and return block headers
type BlocksFinder struct {
	storage storage.Headers
}

// NewBlockFinder constructs a new block finder
func NewBlockFinder(storage storage.Headers) Blocks {
	return &BlocksFinder{storage: storage}
}

// ByHeightFrom returns the block header by height.
func (b *BlocksFinder) ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error) {
	if header == nil {
		byHeight, err := b.storage.ByHeight(height)
		if err != nil {
			return nil, err
		}
		return byHeight, nil
	}

	if header.Height == height {
		return header, nil
	}

	if height > header.Height {
		// TODO figure out min hight and enforce it to be bigger than min hight
		return nil, &errors.InvalidBlockHeightError{MaxHeight: header.Height, MinHeight: 0, RequestedHeight: height}
	}

	id := header.ParentID

	// TODO why we traverse instead of b.storage.ByHeight()
	// travel chain back
	for {
		// recent block should be in cache so this is supposed to be fast
		parent, err := b.storage.ByBlockID(id)
		if err != nil {
			return nil, &errors.BlockFinderFailure{Err: fmt.Errorf("cannot retrieve block parent: %w", err)}
		}
		if parent.Height == height {
			return parent, nil
		}

		_, err = b.storage.ByHeight(parent.Height)
		// if height isn't finalized, move to parent
		if err != nil && errors.Is(err, storage.ErrNotFound) {
			id = parent.ParentID
			continue
		}
		// any other error bubbles up
		if err != nil {
			return nil, &errors.BlockFinderFailure{Err: fmt.Errorf("cannot retrieve block parent: %w", err)}
		}
		//if parent is finalized block, we can just use finalized chain
		// to get desired height
		return b.storage.ByHeight(height)
	}
}
