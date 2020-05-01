package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
)

const blockStateTimeout = 60 * time.Second

type BlockState struct {
	// TODO add locks to prevent concurrent map access bugs
	blocksByID        map[flow.Identifier]*messages.BlockProposal
	finalizedByHeight map[uint64]*messages.BlockProposal
	highestFinalized  uint64
	highestProposed   uint64
}

func (bs *BlockState) Add(b *messages.BlockProposal) {
	if bs.blocksByID == nil {
		bs.blocksByID = make(map[flow.Identifier]*messages.BlockProposal)
	}
	bs.blocksByID[b.Header.ID()] = b
	bs.highestProposed = b.Header.Height

	// add confirmations
	confirmsHeight := b.Header.Height - 3
	if b.Header.Height >= 3 && confirmsHeight > bs.highestFinalized {
		if bs.finalizedByHeight == nil {
			bs.finalizedByHeight = make(map[uint64]*messages.BlockProposal)
		}
		// put all ancestors into `finalizedByHeight`
		ancestor, ok := b, true
		for ancestor.Header.Height > bs.highestFinalized {
			h := ancestor.Header.Height

			// if ancestor is confirmed put it into the finalized map
			if h <= confirmsHeight {
				bs.finalizedByHeight[h] = ancestor

				// if ancestor confirms a heigher height than highestFinalized, increase highestFinalized
				if h > bs.highestFinalized {
					bs.highestFinalized = h
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
}

// WaitForFirstFinalized waits until the first block is finalized
func (bs *BlockState) WaitForFirstFinalized(t *testing.T) *messages.BlockProposal {
	require.Eventually(t, func() bool {
		return bs.highestFinalized > 0
	}, blockStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive first finalized block within %v seconds", blockStateTimeout))

	return bs.finalizedByHeight[bs.highestFinalized]
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

// HighestFinalized returns the highest finalized block and a boolean indicating whether a highest finalized block
// exists
func (bs *BlockState) HighestFinalized() (*messages.BlockProposal, bool) {
	if bs.highestFinalized == 0 {
		return nil, false
	}
	block, ok := bs.finalizedByHeight[bs.highestFinalized]
	return block, ok
}
