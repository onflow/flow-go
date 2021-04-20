package fork

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) error

// TraverseBackward traverses the given fork (specified by block ID `forkHead`)
// in the order of decreasing height. The `terminal` defines when the traversal
// stops. The `visitor` callback is called for each block in this segment.
func TraverseBackward(headers storage.Headers, forkHead flow.Identifier, visitor onVisitBlock, terminal Terminal) error {
	lowestHeightToVisit, sanityChecker, err := terminal.Translate2Height(headers)
	if err != nil {
		return fmt.Errorf("error evaluating terminal: %w", err)
	}

	startBlock, err := headers.ByBlockID(forkHead)
	if err != nil {
		return fmt.Errorf("could not fork head with if  %x: %w", forkHead, err)
	}
	if startBlock.Height < lowestHeightToVisit {
		return nil
	}

	return unsafeTraverse(headers, startBlock, visitor, lowestHeightToVisit, sanityChecker)
}

// unsafeTraverse implements the fork traversal in the order of decreasing height.
// It is unsafe because:
// * always calls the `visitor` on the `block`, _before_ it checks whether to stop
// * if `block` has a lower height than lowestHeightToVisit, the traversal keeps
//   going until we hit the root block
func unsafeTraverse(headers storage.Headers, block *flow.Header, visitor onVisitBlock, lowestHeightToVisit uint64, sanityChecker sanityCheckLowestVisitedBlock) error {
	for {
		err := visitor(block)
		if err != nil {
			return err
		}

		if block.Height == lowestHeightToVisit {
			err = sanityChecker(block)
			if err != nil {
				return fmt.Errorf("lowest visited block at height %d lowestHeightToVisit failed sanity check: %w", lowestHeightToVisit, err)
			}
			return nil
		}

		block, err = headers.ByBlockID(block.ParentID)
		if err != nil {
			return fmt.Errorf("could not get block header %x: %w", block.ParentID, err)
		}
	}
}

// TraverseForward traverses the given fork (specified by block ID `forkHead`)
// in the order of increasing height. The `terminal` defines when the traversal
// begins. The `visitor` callback is called for each block in this segment.
func TraverseForward(headers storage.Headers,
	forkHead flow.Identifier,
	visitor onVisitBlock,
	terminal Terminal,
) error {
	lowestHeightToVisit, sanityChecker, err := terminal.Translate2Height(headers)
	if err != nil {
		return fmt.Errorf("error evaluating terminal: %w", err)
	}

	startBlock, err := headers.ByBlockID(forkHead)
	if err != nil {
		return fmt.Errorf("could not fork head with if  %x: %w", forkHead, err)
	}
	if startBlock.Height < lowestHeightToVisit {
		return nil
	}

	blocks := make([]*flow.Header, 0, startBlock.Height-lowestHeightToVisit+1)
	err = unsafeTraverse(headers,
		startBlock,
		func(header *flow.Header) error {
			blocks = append(blocks, header)
			return nil
		},
		lowestHeightToVisit,
		sanityChecker,
	)
	if err != nil {
		return err
	}

	for i := len(blocks) - 1; i >= 0; i-- {
		err = visitor(blocks[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// functor that will be called on each block header when traversing blocks.
type sanityCheckLowestVisitedBlock func(header *flow.Header) error

// Terminal specifies the terminal condition for traversing a fork.
// Any condition that can be converted to a block height can be
// represented as a terminal.
type Terminal interface {
	// Translate2Height converts the terminal condition to:
	//  * first parameter: lowest height that should be visited
	//  * second parameter: a sanity check for the lowest visited block
	//                      (e.g. reaching a block with a specific ID)
	//  * third parameter: an error if converting the terminal condition failed
	Translate2Height(headers storage.Headers) (uint64, sanityCheckLowestVisitedBlock, error)
}
