package common

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

const receiptStateTimeout = 60 * time.Second

type ReceiptState struct {
	// receipts contains execution receipts are indexed by blockID, then by executorID
	// TODO add lock to prevent concurrent map access bugs
	receipts map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt
}

func (rs *ReceiptState) Add(er *flow.ExecutionReceipt) {
	if rs.receipts == nil {
		// TODO: initialize this map in constructor
		rs.receipts = make(map[flow.Identifier]map[flow.Identifier]*flow.ExecutionReceipt)
	}

	if rs.receipts[er.ExecutionResult.BlockID] == nil {
		rs.receipts[er.ExecutionResult.BlockID] = make(map[flow.Identifier]*flow.ExecutionReceipt)
	}

	rs.receipts[er.ExecutionResult.BlockID][er.ExecutorID] = er
}

// WaitForReceiptFromAny waits for an execution receipt for the given blockID from any execution node and returns it
func (rs *ReceiptState) WaitForReceiptFromAny(t *testing.T, blockID flow.Identifier) *flow.ExecutionReceipt {
	require.Eventually(t, func() bool {
		return len(rs.receipts[blockID]) > 0
	}, receiptStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from any node within %v seconds", blockID,
			receiptStateTimeout))
	for _, r := range rs.receipts[blockID] {
		return r
	}
	panic("there needs to be an entry in rs.receipts[blockID]")
}

// WaitForReceiptFrom waits for an execution receipt for the given blockID and the given executorID and returns it
func (rs *ReceiptState) WaitForReceiptFrom(t *testing.T, blockID, executorID flow.Identifier) *flow.ExecutionReceipt {
	var r *flow.ExecutionReceipt
	require.Eventually(t, func() bool {
		var ok bool
		r, ok = rs.receipts[blockID][executorID]
		return ok
	}, receiptStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive execution receipt for block ID %x from %x within %v seconds", blockID, executorID,
			receiptStateTimeout))
	return r
}

// WaitUntilFinalizedStateCommitmentChanged waits until a different state commitment for a finalized block is received
// compared to the latest one from any execution node and returns the corresponding block and execution receipt
func WaitUntilFinalizedStateCommitmentChanged(t *testing.T, bs *BlockState, rs *ReceiptState,
	qualifiers ...func(receipt flow.ExecutionReceipt) bool) (*messages.BlockProposal,
	*flow.ExecutionReceipt) {

	// get the state commitment for the highest finalized block
	initialFinalizedSC := unittest.GenesisStateCommitment
	var err error
	b1, ok := bs.HighestFinalized()
	if ok {
		r1 := rs.WaitForReceiptFromAny(t, b1.Header.ID())
		initialFinalizedSC, err = r1.ExecutionResult.FinalStateCommitment()
		require.NoError(t, err)
	}

	initFinalizedheight := b1.Header.Height
	currentHeight := initFinalizedheight + 1

	currentID := b1.Header.ID()
	var b2 *messages.BlockProposal
	var r2 *flow.ExecutionReceipt
	require.Eventually(t, func() bool {
		var ok bool
		b2, ok = bs.finalizedByHeight[currentHeight]
		if !ok {
			return false
		}
		currentID = b2.Header.ID()
		r2 = rs.WaitForReceiptFromAny(t, b2.Header.ID())
		r2finalState, err := r2.ExecutionResult.FinalStateCommitment()
		require.NoError(t, err)
		if initialFinalizedSC == r2finalState {
			// received a new execution result for the next finalized block, but it has the same final state commitment
			// check the next finalized block
			currentHeight++
			return false
		}

		for _, qualifier := range qualifiers {
			if !qualifier(*r2) {
				return false
			}
		}

		return true
	}, receiptStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive an execution receipt with a different state commitment from %x within %v seconds,"+
			" initial finalized height: %v "+
			" last block checked height %v, last block checked ID %x", initialFinalizedSC, receiptStateTimeout,
			initFinalizedheight,
			currentHeight, currentID))

	return b2, r2
}

// WithMinimumChunks creates a qualifier that returns true if receipt has the specified minimum number of chunks.
func WithMinimumChunks(chunkNum int) func(flow.ExecutionReceipt) bool {
	return func(receipt flow.ExecutionReceipt) bool {
		return len(receipt.ExecutionResult.Chunks) >= chunkNum
	}
}
