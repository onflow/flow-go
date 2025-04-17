package environment

import (
	"fmt"

	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/cadence/runtime"

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
func (finder *BlocksFinder) ByHeightFrom(height uint64, header *flow.Header) (*flow.Header, error) {
	if header == nil {
		byHeight, err := finder.storage.ByHeight(height)
		if err != nil {
			return nil, err
		}
		return byHeight, nil
	}

	if header.Height == height {
		return header, nil
	}

	if height > header.Height {
		// TODO figure out min height and enforce it to be bigger than min height
		minHeight := 0
		msg := fmt.Sprintf("requested height (%d) is not in the range(%d, %d)", height, minHeight, header.Height)
		err := errors.NewValueErrorf(fmt.Sprint(height), msg)
		return nil, fmt.Errorf("cannot retrieve block parent: %w", err)
	}

	id := header.ParentID

	// travel chain back
	for {
		// recent block should be in cache so this is supposed to be fast
		parent, err := finder.storage.ByBlockID(id)
		if err != nil {
			failure := errors.NewBlockFinderFailure(err)
			return nil, fmt.Errorf("cannot retrieve block parent: %w", failure)
		}
		if parent.Height == height {
			return parent, nil
		}

		_, err = finder.storage.ByHeight(parent.Height)
		// if height isn't finalized, move to parent
		if err != nil && errors.Is(err, storage.ErrNotFound) {
			id = parent.ParentID
			continue
		}
		// any other error bubbles up
		if err != nil {
			failure := errors.NewBlockFinderFailure(err)
			return nil, fmt.Errorf("cannot retrieve block parent: %w", failure)
		}
		//if parent is finalized block, we can just use finalized chain
		// to get desired height
		return finder.storage.ByHeight(height)
	}
}

// NoopBlockFinder implements the Blocks interface. It is used in the
// bootstrapping process.
type NoopBlockFinder struct{}

func (NoopBlockFinder) ByHeightFrom(_ uint64, _ *flow.Header) (*flow.Header, error) {
	return nil, nil
}

func runtimeBlockFromHeader(header *flow.Header) runtime.Block {
	return runtime.Block{
		Height:    header.Height,
		View:      header.View,
		Hash:      stdlib.BlockHash(header.ID()),
		Timestamp: int64(header.Timestamp * 1e6),
	}
}
