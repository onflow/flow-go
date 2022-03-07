package blockstate

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
	sync.RWMutex
	blocksByID        map[flow.Identifier]*messages.BlockProposal
	finalizedByHeight map[uint64]*messages.BlockProposal
	highestFinalized  uint64
	highestProposed   uint64
	highestSealed     *messages.BlockProposal
}

func NewBlockState() *BlockState {
	return &BlockState{
		RWMutex:           sync.RWMutex{},
		blocksByID:        make(map[flow.Identifier]*messages.BlockProposal),
		finalizedByHeight: make(map[uint64]*messages.BlockProposal),
	}
}

func (bs *BlockState) Add(t *testing.T, b *messages.BlockProposal) {
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

	bs.processAncestors(t, b, confirmsHeight)
}

// processAncestors checks whether ancestors of block are within the confirming height, and finalizes
// them if that is the case.
// It also processes the seals of blocks being finalized.
func (bs *BlockState) processAncestors(t *testing.T, b *messages.BlockProposal, confirmsHeight uint64) {
	// puts this block proposal and all ancestors into `finalizedByHeight`
	t.Logf("new height arrived: %d\n", b.Header.Height)
	ancestor, ok := b, true
	for ancestor.Header.Height > bs.highestFinalized {
		heightDistance := b.Header.Height - ancestor.Header.Height
		viewDistance := b.Header.View - ancestor.Header.View
		if ancestor.Header.Height <= confirmsHeight {
			// Since we are running on a trusted setup on localnet, when we receive block height b.Header.Height,
			// it can finalize all ancestor blocks at height < confirmsHeight given the following conditions both satisfied:
			// (1) we already received ancestor block.
			// (2) there is no fork: the view distance between received block and ancestor block is the same as their height distance.
			// for instance, if we have received block 10, 11, 12, 13, and they have 3 views apart and 3 heights apart.
			// We can say block 10 is finalized, without receiving any blocks prior to block 10.
			if viewDistance == heightDistance {
				finalized := ancestor

				bs.finalizedByHeight[finalized.Header.Height] = finalized
				if finalized.Header.Height > bs.highestFinalized { // updates highestFinalized height
					bs.highestFinalized = finalized.Header.Height
				}
				t.Logf("height %d finalized %d, highest finalized %d \n",
					b.Header.Height,
					finalized.Header.Height,
					bs.highestFinalized)
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
			} else {
				t.Logf("fork detected: view distance (%d) between received block and ancestor is not same as their height distance (%d)\n",
					viewDistance, heightDistance)
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

// WaitForHighestFinalizedProgress waits until last finalized height progresses, e.g., if
// latest finalized block is 10, this will wait until any block height higher than 10 is finalized, and
// returns that block.
func (bs *BlockState) WaitForHighestFinalizedProgress(t *testing.T) *messages.BlockProposal {
	currentFinalized := bs.highestFinalized
	require.Eventually(t, func() bool {
		bs.RLock() // avoiding concurrent map access
		defer bs.RUnlock()

		t.Logf("checking highest finalized: %d, highest proposed: %d\n", bs.highestFinalized, bs.highestProposed)
		return bs.highestFinalized > currentFinalized
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive progress on highest finalized height (%v) from (%v) within %v seconds",
			bs.highestFinalized,
			currentFinalized,
			blockStateTimeout))

	return bs.finalizedByHeight[bs.highestFinalized]
}

// WaitUntilNextHeightFinalized waits until the next block height that will be proposed is finalized: If the latest
// proposed block has height 13, and the latest finalized block is 10, this will wait until block height 14 is finalized
func (bs *BlockState) WaitUntilNextHeightFinalized(t *testing.T) *messages.BlockProposal {
	currentProposed := bs.highestProposed
	require.Eventually(t, func() bool {
		bs.RLock() // avoiding concurrent map access
		defer bs.RUnlock()

		return bs.finalizedByHeight[currentProposed+1] != nil
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
		bs.RLock() // avoiding concurrent map access
		defer bs.RUnlock()

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
	bs.RLock() // avoiding concurrent map access
	defer bs.RUnlock()

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
				t.Logf("waiting for sealed height (%d/%d)", bs.highestSealed.Header.Height, height)
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

func (bs *BlockState) WaitForSealedView(t *testing.T, view uint64) *messages.BlockProposal {
	timeout := 3 * blockStateTimeout
	require.Eventually(t,
		func() bool {
			if bs.highestSealed != nil {
				t.Logf("waiting for sealed view (%d/%d)", bs.highestSealed.Header.View, view)
			}
			return bs.highestSealed != nil && bs.highestSealed.Header.View >= view
		},
		timeout,
		100*time.Millisecond,
		fmt.Sprintf("did not receive sealed block for view (%v) within %v seconds", view, timeout))
	return bs.highestSealed
}

func (bs *BlockState) FinalizedHeight(currentHeight uint64) (*messages.BlockProposal, bool) {
	bs.RLock()
	defer bs.RUnlock()

	blk, ok := bs.finalizedByHeight[currentHeight]
	return blk, ok
}
