package state

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Traverse traverses a chain segment beginning with the start block (inclusive)
// and ending with the end block (inclusive). Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
func Traverse(headers storage.Headers, start, end flow.Identifier, callback func(header *flow.Header) error) error {

	nextID := start
	for {
		// retrieve the next block in the segment and pass it to the callback
		next, err := headers.ByBlockID(nextID)
		if err != nil {
			return fmt.Errorf("could not get segment block (id=%x): %w", nextID, err)
		}
		err = callback(next)
		if err != nil {
			return fmt.Errorf("error in callback: %w", err)
		}

		if nextID == end {
			return nil
		}
		nextID = next.ParentID
	}
}
