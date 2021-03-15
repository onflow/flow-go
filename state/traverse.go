package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be get called on each block header when traversing blocks.
type onBlockTraverse = func(header *flow.Header) (bool, error)

// TraverseBlocksBackwards traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
func TraverseBlocksBackwards(headers storage.Headers, startBlockID flow.Identifier, traverse onBlockTraverse) error {
	ancestorID := startBlockID
	for {
		ancestor, err := headers.ByBlockID(startBlockID)
		if err != nil {
			return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}

		shouldContinue, err := traverse(ancestor)
		if !shouldContinue {
			break
		}
		if err != nil {
			return err
		}
		ancestorID = ancestor.ParentID
	}
	return nil
}
