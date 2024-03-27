package lib

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

const blockStateTimeout = 120 * time.Second

type BlockState struct {
	sync.RWMutex
	blocksByID        map[flow.Identifier]*flow.Block
	blocksByHeight    map[uint64][]*flow.Block
	finalizedByHeight map[uint64]*flow.Block
	highestFinalized  uint64
	highestProposed   *flow.Block
	highestSealed     *flow.Block
}

func NewBlockState() *BlockState {
	return &BlockState{
		RWMutex:           sync.RWMutex{},
		blocksByID:        make(map[flow.Identifier]*flow.Block),
		blocksByHeight:    make(map[uint64][]*flow.Block),
		finalizedByHeight: make(map[uint64]*flow.Block),
	}
}

func (bs *BlockState) ByBlockID(id flow.Identifier) (*flow.Block, bool) {
	block, ok := bs.blocksByID[id]
	return block, ok
}

// WaitForHalt attempts to detect when consensus has halted by observing a certain duration
// pass without progress (proposals with newer views).
func (bs *BlockState) WaitForHalt(t *testing.T, requiredDurationWithoutProgress, tick, timeout time.Duration) {
	timeSinceLastProgress := time.Duration(0)
	lastView := bs.HighestProposedView()
	start := time.Now()
	lastProgress := start

	t.Logf("waiting for halt: lastView=%d", lastView)

	ticker := time.NewTicker(tick)
	timer := time.NewTimer(timeout)
	defer ticker.Stop()
	defer timer.Stop()
	for timeSinceLastProgress < requiredDurationWithoutProgress {
		select {
		case <-timer.C:
			t.Fatalf("failed to observe progress halt after %s, requiring %s without progress to succeed", timeout, requiredDurationWithoutProgress)
		case <-ticker.C:
		}

		latestView := bs.HighestProposedView()
		if latestView > lastView {
			lastView = latestView
			lastProgress = time.Now()
		}
		timeSinceLastProgress = time.Now().Sub(lastProgress)
	}
	t.Logf("successfully observed progress halt for %s after %s of waiting", requiredDurationWithoutProgress, time.Since(start))
}

func (bs *BlockState) Add(t *testing.T, msg *messages.BlockProposal) {
	b := msg.Block.ToInternal()
	bs.Lock()
	defer bs.Unlock()

	bs.blocksByID[b.Header.ID()] = b
	bs.blocksByHeight[b.Header.Height] = append(bs.blocksByHeight[b.Header.Height], b)
	if bs.highestProposed == nil {
		bs.highestProposed = b
	} else if b.Header.View > bs.highestProposed.Header.View {
		bs.highestProposed = b
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

func (bs *BlockState) WaitForBlockById(t *testing.T, blockId flow.Identifier) *flow.Block {
	var blockProposal *flow.Block

	require.Eventually(t, func() bool {
		bs.RLock()
		defer bs.RUnlock()

		if block, ok := bs.blocksByID[blockId]; !ok {
			t.Logf("%v pending for block id: %x\n", time.Now().UTC(), blockId)
			return false
		} else {
			blockProposal = block
			return true
		}

	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive requested block id (%x) within %v seconds", blockId, blockStateTimeout))

	return blockProposal
}

// WaitForBlocksByHeight waits until a block at height is observed, and returns all observed blocks at that height.
func (bs *BlockState) WaitForBlocksByHeight(t *testing.T, height uint64) []*flow.Block {
	var blockProposals []*flow.Block

	require.Eventually(t, func() bool {
		bs.RLock()
		defer bs.RUnlock()

		if blocks, ok := bs.blocksByHeight[height]; !ok || len(blocks) == 0 {
			t.Logf("%v pending for blocks height: %x\n", time.Now().UTC(), height)
			return false
		} else {
			blockProposals = blocks
			return true
		}

	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive requested blocks height (%d) within %v seconds", height, blockStateTimeout))

	return blockProposals
}

// processAncestors checks whether ancestors of block are within the confirming height, and finalizes
// them if that is the case.
// It also processes the seals of blocks being finalized.
func (bs *BlockState) processAncestors(t *testing.T, b *flow.Block, confirmsHeight uint64) {
	// puts this block proposal and all ancestors into `finalizedByHeight`
	t.Logf("%v new height arrived: %d\n", time.Now().UTC(), b.Header.Height)
	ancestor := b
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
				t.Logf("%v height %d finalized %d, highest finalized %d \n",
					time.Now().UTC(),
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
				t.Logf("%v fork detected: view distance (%d) between received block and ancestor is not same as their height distance (%d)\n",
					time.Now().UTC(), viewDistance, heightDistance)
			}

		}

		// find parent
		var ok bool
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
func (bs *BlockState) WaitForHighestFinalizedProgress(t *testing.T, currentFinalized uint64) *flow.Block {
	require.Eventually(t, func() bool {
		bs.RLock()
		defer bs.RUnlock()

		t.Logf("%v checking highest finalized: %d, highest proposed: %d\n", time.Now().UTC(), bs.highestFinalized, bs.highestProposed)
		return bs.highestFinalized > currentFinalized
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive progress on highest finalized height (%v) from (%v) within %v seconds",
			bs.highestFinalized,
			currentFinalized,
			blockStateTimeout))

	bs.RLock()
	defer bs.RUnlock()
	return bs.finalizedByHeight[bs.highestFinalized]
}

// WaitUntilNextHeightFinalized waits until the next block height that will be proposed is finalized: If the latest
// proposed block has height 13, and the latest finalized block is 10, this will wait until block height 14 is finalized
func (bs *BlockState) WaitUntilNextHeightFinalized(t *testing.T, currentProposed uint64) *flow.Block {
	require.Eventually(t, func() bool {
		bs.RLock()
		defer bs.RUnlock()

		return bs.finalizedByHeight[currentProposed+1] != nil
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized block for next block height (%v) within %v seconds", currentProposed+1,
			blockStateTimeout))

	bs.RLock()
	defer bs.RUnlock()
	return bs.finalizedByHeight[currentProposed+1]
}

// WaitForFinalizedChild waits until any child of the passed parent will be finalized: If the latest proposed block has
// height 13, and the latest finalized block is 10 and we are waiting for the first finalized child of a parent at
// height 11, this will wait until block height 12 is finalized
func (bs *BlockState) WaitForFinalizedChild(t *testing.T, parent *flow.Block) *flow.Block {
	require.Eventually(t, func() bool {
		bs.RLock()
		defer bs.RUnlock()

		_, ok := bs.finalizedByHeight[parent.Header.Height+1]
		return ok
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive finalized child block for parent block height %v within %v seconds",
			parent.Header.Height, blockStateTimeout))

	bs.RLock()
	defer bs.RUnlock()
	return bs.finalizedByHeight[parent.Header.Height+1]
}

// HighestFinalized returns the highest finalized block after genesis and a boolean indicating whether a highest
// finalized block exists
func (bs *BlockState) HighestFinalized() (*flow.Block, bool) {
	bs.RLock()
	defer bs.RUnlock()

	if bs.highestFinalized == 0 {
		return nil, false
	}
	block, ok := bs.finalizedByHeight[bs.highestFinalized]
	return block, ok
}

// HighestProposedHeight returns the height of the highest proposed block.
func (bs *BlockState) HighestProposedHeight() uint64 {
	bs.RLock()
	defer bs.RUnlock()
	return bs.highestProposed.Header.Height
}

// HighestProposedView returns the view of the highest proposed block.
func (bs *BlockState) HighestProposedView() uint64 {
	bs.RLock()
	defer bs.RUnlock()
	return bs.highestProposed.Header.View
}

// HighestFinalizedHeight returns the height of the highest finalized block.
func (bs *BlockState) HighestFinalizedHeight() uint64 {
	bs.RLock()
	defer bs.RUnlock()
	return bs.highestFinalized
}

// WaitForSealed returns the sealed block after a certain height has been sealed.
func (bs *BlockState) WaitForSealed(t *testing.T, height uint64) *flow.Block {
	require.Eventually(t,
		func() bool {
			bs.RLock()
			defer bs.RUnlock()

			if bs.highestSealed != nil {
				t.Logf("%v waiting for sealed height (%d/%d), last finalized %d", time.Now().UTC(), bs.highestSealed.Header.Height, height, bs.highestFinalized)
			}
			return bs.highestSealed != nil && bs.highestSealed.Header.Height >= height
		},
		blockStateTimeout,
		100*time.Millisecond,
		fmt.Sprintf("did not receive sealed block for height (%v) within %v seconds", height, blockStateTimeout))

	bs.RLock()
	defer bs.RUnlock()
	return bs.highestSealed
}

func (bs *BlockState) WaitForSealedView(t *testing.T, view uint64) *flow.Block {
	require.Eventually(t,
		func() bool {
			bs.RLock()
			defer bs.RUnlock()

			if bs.highestSealed != nil {
				t.Logf("%v waiting for sealed view (%d/%d), last finalized %d", time.Now().UTC(), bs.highestSealed.Header.Height, view, bs.highestFinalized)
			}
			return bs.highestSealed != nil && bs.highestSealed.Header.View >= view
		},
		blockStateTimeout,
		100*time.Millisecond,
		fmt.Sprintf("did not receive sealed block for view (%v) within %v seconds", view, blockStateTimeout))

	bs.RLock()
	defer bs.RUnlock()
	return bs.highestSealed
}

func (bs *BlockState) HighestSealed() (*flow.Block, bool) {
	bs.RLock()
	defer bs.RUnlock()

	if bs.highestSealed == nil {
		return nil, false
	}
	return bs.highestSealed, true
}

func (bs *BlockState) FinalizedHeight(currentHeight uint64) (*flow.Block, bool) {
	bs.RLock()
	defer bs.RUnlock()

	blk, ok := bs.finalizedByHeight[currentHeight]
	return blk, ok
}
