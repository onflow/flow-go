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
	startBlock, err := headers.ByBlockID(forkHead)
	if err != nil {
		return fmt.Errorf("could not retrieve fork head with id %x: %w", forkHead, err)
	}
	lowestHeightToVisit, err := terminal.LowestHeightToVisit(headers)
	if err != nil {
		return fmt.Errorf("error determinging terminal height: %w", err)
	}
	if startBlock.Height < lowestHeightToVisit {
		return nil
	}

	lowestBlock, err := unsafeTraverse(headers, startBlock, visitor, lowestHeightToVisit)
	if err != nil {
		return fmt.Errorf("traversing fork aborted: %w", err)
	}

	err = terminal.ConfirmTerminalReached(headers, lowestBlock)
	if err != nil {
		return fmt.Errorf("lowest visited block at height %d failed sanity check: %w", lowestHeightToVisit, err)
	}

	return nil
}

// TraverseForward traverses the given fork (specified by block ID `forkHead`)
// in the order of increasing height. The `terminal` defines when the traversal
// begins. The `visitor` callback is called for each block in this segment.
func TraverseForward(headers storage.Headers,
	forkHead flow.Identifier,
	visitor onVisitBlock,
	terminal Terminal,
) error {
	startBlock, err := headers.ByBlockID(forkHead)
	if err != nil {
		return fmt.Errorf("could not retrieve fork head with id %x: %w", forkHead, err)
	}
	lowestHeightToVisit, err := terminal.LowestHeightToVisit(headers)
	if err != nil {
		return fmt.Errorf("error determinging terminal height: %w", err)
	}
	if startBlock.Height < lowestHeightToVisit {
		return nil
	}

	blocks := make([]*flow.Header, 0, startBlock.Height-lowestHeightToVisit+1)
	lowestBlock, err := unsafeTraverse(headers,
		startBlock,
		func(header *flow.Header) error {
			blocks = append(blocks, header)
			return nil
		},
		lowestHeightToVisit,
	)
	if err != nil {
		return fmt.Errorf("traversing fork aborted: %w", err)
	}

	err = terminal.ConfirmTerminalReached(headers, lowestBlock)
	if err != nil {
		return fmt.Errorf("lowest visited block at height %d failed sanity check: %w", lowestHeightToVisit, err)
	}

	for i := len(blocks) - 1; i >= 0; i-- {
		block := blocks[i]
		err = visitor(block)
		if err != nil {
			return fmt.Errorf("visitor errored on block %x at height %d: %w", block.ID(), block.Height, err)
		}
	}

	return nil
}

// unsafeTraverse implements the fork traversal in the order of decreasing height.
// It is unsafe because:
// it assumes the stop condition for the for-loop has been pre-checked,
// which is `startBlock.Height < lowestHeightToVisit`. With the pre-check,
// it is guaranteed the for loop will stop eventually.
// In other words, this unsafe function should only be called after the pre-check.
// The `TraverseBackward` and `TraverseForward` are "safe" functions since they
// do the pre-check before calling the `unsafeTraverse`
func unsafeTraverse(headers storage.Headers, block *flow.Header, visitor onVisitBlock, lowestHeightToVisit uint64) (*flow.Header, error) {
	for {
		err := visitor(block)
		if err != nil {
			return nil, fmt.Errorf("visitor errored on block %x at height %d: %w", block.ID(), block.Height, err)
		}

		if block.Height == lowestHeightToVisit {
			return block, nil
		}

		block, err = headers.ByBlockID(block.ParentID)
		if err != nil {
			return nil, fmt.Errorf("failed to revtrieve block header %x: %w", block.ParentID, err)
		}
	}
}
