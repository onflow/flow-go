package common

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

const blockStateTimeout = 60 * time.Second

type BlockState struct {
	sync.Mutex
	blocksByID        map[flow.Identifier]*messages.BlockProposal
	finalizedByHeight map[uint64]*messages.BlockProposal
	highestFinalized  uint64
	highestProposed   uint64
	highestSealed     *messages.BlockProposal
}

func NewBlockState() *BlockState {
	return &BlockState{
		Mutex:             sync.Mutex{},
		blocksByID:        make(map[flow.Identifier]*messages.BlockProposal),
		finalizedByHeight: make(map[uint64]*messages.BlockProposal),
	}
}

func (bs *BlockState) Add(b *messages.BlockProposal) {
	bs.Lock()
	defer bs.Unlock()

	bs.blocksByID[b.Header.ID()] = b
	if b.Header.Height > bs.highestProposed {
		bs.highestProposed = b.Header.Height
	}

	if b.Header.Height < 3 {
		return
	}

	confirmsHeight := b.Header.Height - uint64(3)
	if confirmsHeight < bs.highestFinalized {
		return
	}

	bs.processAncestors(b, confirmsHeight)
	bs.updateHighestFinalizedHeight()
}

// processAncestors checks whether ancestors of block are within the confirming height, and finalizes
// them if that is the case.
// It also processes the seals of blocks being finalized.
func (bs *BlockState) processAncestors(b *messages.BlockProposal, confirmsHeight uint64) {
	// puts this block proposal and all ancestors into `finalizedByHeight`
	ancestor, ok := b, true
	for ancestor.Header.Height > bs.highestFinalized {
		h := ancestor.Header.Height

		// if ancestor is confirmed put it into the finalized map
		if h <= confirmsHeight {

			finalized := ancestor
			bs.finalizedByHeight[h] = finalized

			// update last sealed height
			for _, seal := range finalized.Payload.Seals {
				sealed, ok := bs.blocksByID[seal.BlockID]
				if !ok {
					continue
				}

				if bs.highestSealed == nil ||
					sealed.Header.Height > bs.highestSealed.Header.Height {
					bs.highestSealed = sealed
				}
			}

		}

		// find parent
		ancestor, ok = bs.blocksByID[ancestor.Header.ParentID]

		// stop if parent not found
		if !ok {
			return
		}
	}
}

// updateHighestFinalizedHeight moves forward the highestFinalized height for the newly finalized blocks.
func (bs *BlockState) updateHighestFinalizedHeight() {
	for {
		// checks whether next height has been finalized and updates highest finalized height
		// if that is the case.
		if _, ok := bs.finalizedByHeight[bs.highestFinalized+1]; !ok {
			return
		}

		bs.highestFinalized++
	}
}

// WaitForFirstFinalized waits until the first block is finalized
func (bs *BlockState) WaitForFirstFinalized(t *testing.T) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		return bs.highestFinalized > 0
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive first finalized block within %v seconds", blockStateTimeout))

	return bs.finalizedByHeight[1]
}

// WaitForNextFinalized waits until the next block is finalized: If the latest proposed block has height 13, and the
// latest finalized block is 10, this will wait until block height 11 is finalized
func (bs *BlockState) WaitForNextFinalized(t *testing.T) *messages.BlockProposal {
	currentFinalized := bs.highestFinalized
	require.Eventually(t, func() bool {
		return bs.highestFinalized > currentFinalized
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive next finalizedblock (height %v) within %v seconds", currentFinalized+1,
			blockStateTimeout))

	return bs.finalizedByHeight[currentFinalized+1]
}

// WaitUntilNextHeightFinalized waits until the next block height that will be proposed is finalized: If the latest
// proposed block has height 13, and the latest finalized block is 10, this will wait until block height 14 is finalized
func (bs *BlockState) WaitUntilNextHeightFinalized(t *testing.T) *messages.BlockProposal {
	currentProposed := bs.highestProposed
	require.Eventually(t, func() bool {
		return bs.highestFinalized > currentProposed
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized block for next block height (%v) within %v seconds", currentProposed+1,
			blockStateTimeout))

	return bs.finalizedByHeight[currentProposed+1]
}

// WaitForFinalizedChild waits until any child of the passed parent will be finalized: If the latest proposed block has
// height 13, and the latest finalized block is 10 and we are waiting for the first finalized child of a parent at
// height 11, this will wait until block height 12 is finalized
func (bs *BlockState) WaitForFinalizedChild(t *testing.T, parent *messages.BlockProposal) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		_, ok := bs.finalizedByHeight[parent.Header.Height+1]
		return ok
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized child block for parent block height %v within %v seconds",
			parent.Header.Height, blockStateTimeout))

	return bs.finalizedByHeight[parent.Header.Height+1]
}

// HighestFinalized returns the highest finalized block after genesis and a boolean indicating whether a highest
// finalized block exists
func (bs *BlockState) HighestFinalized() (*messages.BlockProposal, bool) {
	if bs.highestFinalized == 0 {
		return nil, false
	}
	block, ok := bs.finalizedByHeight[bs.highestFinalized]
	return block, ok
}

// WaitForSealed returns the sealed block after a certain height has been sealed.
func (bs *BlockState) WaitForSealed(t *testing.T, height uint64) *messages.BlockProposal {
	require.Eventually(t,
		func() bool {
			if bs.highestSealed != nil {
				fmt.Println("waiting for sealed", bs.highestSealed.Header.Height, height)
			}
			return bs.highestSealed != nil && bs.highestSealed.Header.Height >= height
		},
		blockStateTimeout,
		100*time.Millisecond,
		fmt.Sprintf("did not receive sealed block for height (%v) within %v seconds", height, blockStateTimeout))

	return bs.highestSealed
}

func (bs *BlockState) HighestSealed() (*messages.BlockProposal, bool) {
	if bs.highestSealed == nil {
		return nil, false
	}
	return bs.highestSealed, true
}
