package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be get called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) (bool, error)

// Traverse traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
func Traverse(headers storage.Headers, startBlockID flow.Identifier, visitor onVisitBlock) error {
	ancestorID := startBlockID
	for {
		ancestor, err := headers.ByBlockID(ancestorID)
		if err != nil {
			return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}

		shouldContinue, err := visitor(ancestor)
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

// TraverseParentFirst traverses a chain segment beginning with the last block (non inclusive)
// and ending with start block
// Implements a recursive descend to last block identified by `stopBlockID` and then calling
// visitor callback for every block way back up to `startBlockID`
func TraverseParentFirst(headers storage.Headers, startBlockID, stopBlockID flow.Identifier,
	visitor func(header *flow.Header) error,
) error {

	if startBlockID == stopBlockID {
		return nil
	}

	ancestor, err := headers.ByBlockID(startBlockID)
	if err != nil {
		return fmt.Errorf("could not get ancestor header (%x): %w", startBlockID, err)
	}

	// descend further down the chain
	err = TraverseParentFirst(headers, ancestor.ParentID, stopBlockID, visitor)
	if err != nil {
		return err
	}

	// now we are on our way back up
	err = visitor(ancestor)
	if err != nil {
		return err
	}
	return nil
}
