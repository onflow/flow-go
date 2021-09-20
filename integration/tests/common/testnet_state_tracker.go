package common

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/integration/tests/common/approvalstate"
	"github.com/onflow/flow-go/integration/tests/common/blockstate"
	"github.com/onflow/flow-go/integration/tests/common/receiptstate"
	"github.com/onflow/flow-go/integration/tests/common/transactionstate"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestnetStateTracker struct {
	ghostTracking    bool
	BlockState       *blockstate.BlockState
	ReceiptState     *receiptstate.ReceiptState
	ApprovalState    *approvalstate.ResultApprovalState
	TransactionState *transactionstate.TransactionState
	MsgState         MsgState
}

// Track starts to track new blocks, execution receipts and individual messages the ghost receives. The context will
// be used to stop tracking
func (tst *TestnetStateTracker) Track(t *testing.T, ctx context.Context, ghost *client.GhostClient) {
	// reset the state for in between tests
	tst.BlockState = blockstate.NewBlockState()
	tst.ReceiptState = receiptstate.NewReceiptState()
	tst.ApprovalState = approvalstate.NewResultApprovalState()
	tst.TransactionState = transactionstate.NewTransactionState()

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
				t.Logf("error subscribing to ghost: %v", err)
			} else {
				retry = false
			}
		case <-ctx.Done():
			return
		}
	}
	tst.TransactionState.Reader = reader

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
				if strings.Contains(err.Error(), "transport is closing") || strings.Contains(err.Error(), "EOF") {
					return
				}
			}

			// don't allow other errors
			require.NoError(t, err, "could not read next message")

			tst.MsgState.Add(sender, msg)

			switch m := msg.(type) {
			case *messages.BlockProposal:
				tst.BlockState.Add(m)
				t.Logf("block proposal received from %s at height %v, view %v: %x",
					sender,
					m.Header.Height,
					m.Header.View,
					m.Header.ID())
			case *flow.ResultApproval:
				tst.ApprovalState.Add(sender, m)
				t.Logf("result approval received from %s for execution result ID %x and chunk index %v",
					sender,
					m.Body.ExecutionResultID,
					m.Body.ChunkIndex)
			case *flow.ExecutionReceipt:
				finalState, err := m.ExecutionResult.FinalStateCommitment()
				require.NoError(t, err)

				tst.ReceiptState.Add(m)
				t.Logf("execution receipts received from %s for block ID %x by executor ID %x with SC %x resultID %x",
					sender,
					m.ExecutionResult.BlockID,
					m.ExecutorID,
					finalState,
					m.ExecutionResult.ID())

			// case *messages.ClusterBlockProposal:
			// 	//tst.TransactionState.
			// 	header := m.Header
			// 	collection := m.Payload.Collection
			// 	t.Logf("got proposal height=%d col_id=%x size=%d", header.Height, collection.ID(), collection.Len())

			// case *flow.CollectionGuarantee:

			// case *flow.TransactionBody:
			// 	t.Log("got tx: ", msg)
			default:
				t.Logf("other msg received from %s: %#v", sender, msg)
				continue
			}
		}
	}()
}

// WaitUntilFinalizedStateCommitmentChanged waits until a different state commitment for a finalized block is received
// compared to the latest one from any execution node and returns the corresponding block and execution receipt
func WaitUntilFinalizedStateCommitmentChanged(t *testing.T, bs *blockstate.BlockState, rs *receiptstate.ReceiptState,
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
	}, receiptstate.StateTimeout, 100*time.Millisecond,
		fmt.Sprintf("did not receive an execution receipt with a different state commitment from %x within %v seconds,"+
			" initial finalized height: %v "+
			" last block checked height %v, last block checked ID %x", initialFinalizedSC, receiptstate.ReceiptTimeout,
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
