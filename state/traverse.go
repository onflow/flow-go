package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be get called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) (bool, error)

// TraverseBackwards traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
func TraverseBackwards(headers storage.Headers, startBlockID flow.Identifier, visitor onVisitBlock) error {
	blockID := startBlockID
	for {
		block, err := headers.ByBlockID(blockID)
		if err != nil {
			return fmt.Errorf("could not get block header (%x): %w", blockID, err)
		}

		shouldContinue, err := visitor(block)
		if !shouldContinue {
			break
		}
		if err != nil {
			return err
		}
		blockID = block.ParentID
	}
	return nil
}

// TraverseForward traverses a chain segment in forward order.
// Implements a recursive descend to genesis block or till we and then calling
// visitor callback for every block way back up to `startBlockID`
func TraverseForward(headers storage.Headers,
	startBlockID flow.Identifier,
	visitor func(header *flow.Header) error,
	shouldContinue func(header *flow.Header) bool,
) error {
	block, err := headers.ByBlockID(startBlockID)
	if err != nil {
		return fmt.Errorf("could not get block header (%x): %w", startBlockID, err)
	}

	if !shouldContinue(block) {
		return nil
	}

	// descend further down the chain
	err = TraverseForward(headers, block.ParentID, visitor, shouldContinue)
	if err != nil {
		return err
	}

	// now we are on our way back up
	err = visitor(block)
	if err != nil {
		return err
	}
	return nil
}
