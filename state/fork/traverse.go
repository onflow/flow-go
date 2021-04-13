package fork

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// functor that will be called on each block header when traversing blocks.
type onVisitBlock = func(header *flow.Header) error

// TraverseBackward traverses a chain segment beginning with the start block (inclusive)
// Blocks are traversed in reverse
// height order, meaning the end block must be an ancestor of the start block.
// The callback is called for each block in this segment.
// Return value of callback is used to decide if it should continue or not.
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

// TraverseForward traverses a chain segment in forward order.
// The algorithm starts at the `forkHead` and walks the chain backwards towards
// the genesis block. The descend continues as long as `shouldVisitParent` returns
// true. Starting with the first block where `shouldVisitParent` returned false,
// the visited blocks are fed into `visitor` in a forward order (order of
// increasing height). The last block that is fed into `visitor` is `forkHead`.
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
type sanityCheckLowestVisitedBlock = func(header *flow.Header) error

type Terminal interface {
	Translate2Height(headers storage.Headers) (uint64, sanityCheckLowestVisitedBlock, error)
}

/*******************************************************************************
    Implementations of different Terminals for fork traversal
*******************************************************************************/
