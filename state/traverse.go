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
// It first traverses backward with the given `forkHead` and `shouldContinue`. Once stopped, it will pass the visited blocks to the given `visitor` in a forward order. The last block that is fed into `visitor` is the `forkHead`.
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

         i := len(blocks) -1
	for i >= 0 {
		err = visitor(blocks[i])
		i--
		if err != nil {
			return err
		}
	}
	return nil
}
