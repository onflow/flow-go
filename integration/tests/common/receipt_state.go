package common

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
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
func WaitUntilFinalizedStateCommitmentChanged(t *testing.T, bs *BlockState, rs *ReceiptState) (*messages.BlockProposal,
	*flow.ExecutionReceipt) {

	// get the state commitment for the highest finalized block
	initialFinalizedSC := flow.StateCommitment{}
	b1, ok := bs.HighestFinalized()
	if ok {
		r1 := rs.WaitForReceiptFromAny(t, b1.Header.ID())
		initialFinalizedSC = r1.ExecutionResult.FinalStateCommit
	}

	currentHeight := b1.Header.Height + 1
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
		if bytes.Compare(initialFinalizedSC, r2.ExecutionResult.FinalStateCommit) == 0 {
			// received a new execution result for the next finalized block, but it has the same final state commitment
			// check the next finalized block
			currentHeight++
			return false
		}
		return true
	}, receiptStateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive an execution receipt with a different state commitment from %x within %v seconds,"+
			" last block checked height %v, last block checked ID %x", initialFinalizedSC, receiptStateTimeout,
			currentHeight, currentID))

	return b2, r2
}
