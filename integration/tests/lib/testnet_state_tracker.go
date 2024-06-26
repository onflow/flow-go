package lib

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type TestnetStateTracker struct {
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
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for retry := true; retry; {
		select {
		case <-timeout:
			require.FailNowf(t, "could not subscribe to ghost", "%v", timeout)
			return
		case <-ticker.C:
			var err error
			reader, err = ghost.Subscribe(context.Background())
			if err != nil {
				t.Logf("%v error subscribing to ghost: %v\n", time.Now().UTC(), err)
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
				t.Logf("%v block proposal received from %s at height %v, view %v: %x\n",
					time.Now().UTC(),
					sender,
					m.Block.Header.Height,
					m.Block.Header.View,
					m.Block.Header.ID())
			case *flow.ResultApproval:
				tst.ApprovalState.Add(sender, m)
				t.Logf("%v result approval received from %s for execution result ID %x and chunk index %v\n",
					time.Now().UTC(),
					sender,
					m.Body.ExecutionResultID,
					m.Body.ChunkIndex)

			case *flow.ExecutionReceipt:
				finalState, err := m.ExecutionResult.FinalStateCommitment()
				require.NoError(t, err)
				tst.ReceiptState.Add(m)
				t.Logf("%v execution receipts received from %s for block ID %x by executor ID %x with final state %x result ID %x chunks %d\n",
					time.Now().UTC(),
					sender,
					m.ExecutionResult.BlockID,
					m.ExecutorID,
					finalState,
					m.ExecutionResult.ID(),
					len(m.ExecutionResult.Chunks))
			case *messages.ChunkDataResponse:
				// consuming this explicitly to avoid logging full msg which is usually very large because of proof
				t.Logf("%x chunk data pack received from %x\n",
					m.ChunkDataPack.ChunkID,
					sender)

			default:
				t.Logf("%v other msg received from %s: %T\n", time.Now().UTC(), sender, msg)
				continue
			}
		}
	}()
}
