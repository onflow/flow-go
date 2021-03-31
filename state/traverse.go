package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) error

// functor that will be called on each block header to know if we should
// continue traversing the chain (specifically, visit the block's parent)
type shouldVisitParent = func(block *flow.Header) bool

// TraverseBackward traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
func TraverseBackward(headers storage.Headers, startBlockID flow.Identifier, visitor onVisitBlock, shouldVisitParent shouldVisitParent) error {
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

		if !shouldVisitParent(block) {
			return nil
		}

		blockID = block.ParentID
	}
}

// TraverseForward traverses a chain segment in forward order.
// The algorithm starts at the `forkHead` and walks the chain backwards towards
// the genesis block. The descend continues as long as `shouldVisitParent` returns
// true. Starting with the first block where `shouldVisitParent` returned false,
// the visited blocks are fed into `visitor` in a forward order (order of
// increasing height). The last block that is fed into `visitor` is `forkHead`.
func TraverseForward(headers storage.Headers,
	forkHead flow.Identifier,
	visitor onVisitBlock,
	shouldVisitParent shouldVisitParent,
) error {
	var blocks []*flow.Header
	err := TraverseBackward(headers, forkHead, func(header *flow.Header) error {
		blocks = append(blocks, header)
		return nil
	}, shouldVisitParent)
	if err != nil {
		return err
	}

	i := len(blocks) - 1
	for i >= 0 {
		err = visitor(blocks[i])
		i--
		if err != nil {
			return err
		}
	}
	return nil
}
