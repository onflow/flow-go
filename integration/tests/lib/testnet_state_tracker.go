package lib

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestnetStateTracker struct {
	ghostTracking bool
	BlockState    *BlockState
	ReceiptState  *ReceiptState
	ApprovalState *ResultApprovalState
	MsgState      *MsgState
}

// Track starts to track new blocks, execution receipts and individual messages the ghost receives. The context will
// be used to stop tracking
func (tst *TestnetStateTracker) Track(t *testing.T, ctx context.Context, ghost *client.GhostClient) {
	// reset the state for in between tests
	tst.BlockState = NewBlockState()
	tst.ReceiptState = NewReceiptState()
	tst.ApprovalState = NewResultApprovalState()
	tst.MsgState = NewMsgState()

	var reader *client.FlowMessageStreamReader

	// subscribe to the ghost
	timeout := time.After(20 * time.Second)
	ticker := time.Tick(100 * time.Millisecond)
	for retry := true; retry; {
		select {
		case <-timeout:
			require.FailNowf(t, "could not subscribe to ghost", "%v", timeout)
			return
		case <-ticker:
			var err error
			reader, err = ghost.Subscribe(context.Background())
			if err != nil {
				t.Logf("error subscribing to ghost: %v\n", err)
			} else {
				retry = false
			}
		case <-ctx.Done():
			return
		}
	}

	// continue with processing of messages in the background
	go func() {
		for {
			sender, msg, err := reader.Next()

			select {
			// don't error if container shuts down
			case <-ctx.Done():
				return
			default:
				// continue with this iteration of the loop
			}

			if err != nil {
				// Connection is closed, therefore there are no other messages and we can return here
				if strings.Contains(err.Error(), "transport is closing") ||
					strings.Contains(err.Error(), "EOF") ||
					strings.Contains(err.Error(), "connection reset by peer") {
					return
				}
			}

			// don't allow other errors
			require.NoError(t, err, "could not read next message")

			tst.MsgState.Add(sender, msg)

			switch m := msg.(type) {
			case *messages.BlockProposal:
				tst.BlockState.Add(t, m)
				t.Logf("block proposal received from %s at height %v, view %v: %x\n",
					sender,
					m.Header.Height,
					m.Header.View,
					m.Header.ID())
			case *flow.ResultApproval:
				tst.ApprovalState.Add(sender, m)
				t.Logf("result approval received from %s for execution result ID %x and chunk index %v\n",
					sender,
					m.Body.ExecutionResultID,
					m.Body.ChunkIndex)
			case *flow.ExecutionReceipt:
				finalState, err := m.ExecutionResult.FinalStateCommitment()
				require.NoError(t, err)

				tst.ReceiptState.Add(m)
				t.Logf("execution receipts received from %s for block ID %x by executor ID %x with SC %x resultID %x\n",
					sender,
					m.ExecutionResult.BlockID,
					m.ExecutorID,
					finalState,
					m.ExecutionResult.ID())
			default:
				t.Logf("other msg received from %s: %#v\n", sender, msg)
				continue
			}
		}
	}()
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
		b2, ok = bs.FinalizedHeight(currentHeight)
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
			" last block checked height %v, last block checked ID %x", initialFinalizedSC, receiptTimeout,
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
