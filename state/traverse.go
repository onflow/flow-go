package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) error

// functor that will be called on each block header to know if we should continue traversing the chain.
type shouldContinue = func(header *flow.Header) bool

// TraverseBackward traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
func TraverseBackward(headers storage.Headers, startBlockID flow.Identifier, visitor onVisitBlock, shouldContinue shouldContinue) error {
	blockID := startBlockID
	// in case we reached genesis
	for {
		block, err := headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block header (%x): %w", blockID, err)
		}

		err = visitor(block)
		if err != nil {
			return err
		}

		if !shouldContinue(block) {
			return nil
		}

		blockID = block.ParentID
	}
}

// TraverseForward traverses a chain segment in forward order.
// Implements a recursive descend starting at the `forkHead` towards the genesis block. The descend continues as long as `shouldContinue` returns true. All visited blocks, which `shouldContinue` returned true for, are fed into the `visitor` callback starting from the block with the lowest height, in order of increasing height. The last block that is fed into `visitor` is the `forkHead`.
func TraverseForward(headers storage.Headers,
	forkHead flow.Identifier,
	visitor onVisitBlock,
	shouldContinue shouldContinue,
) error {
	var blocks []*flow.Header
	err := TraverseBackward(headers, forkHead, func(header *flow.Header) error {
		blocks = append(blocks, header)
		return nil
	}, shouldContinue)
	if err != nil {
		return err
	}

	for i := range blocks {
		err = visitor(blocks[len(blocks)-i-1])
		if err != nil {
			return err
		}
	}
	return nil
}
