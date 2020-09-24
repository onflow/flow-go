package common

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
	ghostTracking bool
	BlockState    BlockState
	ReceiptState  ReceiptState
	ApprovalState ResultApprovalState
	MsgState      MsgState
}

// Track starts to track new blocks, execution receipts and individual messages the ghost receives. The context will
// be used to stop tracking
func (tst *TestnetStateTracker) Track(t *testing.T, ctx context.Context, ghost *client.GhostClient) {
	// reset the state for in between tests
	tst.BlockState = BlockState{}
	tst.ReceiptState = ReceiptState{}

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

			if err != nil && strings.Contains(err.Error(), "transport is closing") {
				return
			}

			// don't allow other errors
			require.NoError(t, err, "could not read next message")

			tst.MsgState.Add(sender, msg)

			switch m := msg.(type) {
			case *messages.BlockProposal:
				tst.BlockState.Add(m)
				t.Logf("block proposal received from %s at height %v: %x",
					sender,
					m.Header.Height,
					m.Header.ID())
			case *flow.ResultApproval:
				tst.ApprovalState.Add(sender, m)
				t.Logf("result approval received from %s for execution result ID %x and chunk index %v",
					sender,
					m.Body.ExecutionResultID,
					m.Body.ChunkIndex)
			case *flow.ExecutionReceipt:
				tst.ReceiptState.Add(m)
				t.Logf("execution receipts received from %s for block ID %x by executor ID %x with SC %x resultID %x",
					sender,
					m.ExecutionResult.BlockID,
					m.ExecutorID,
					m.ExecutionResult.FinalStateCommit,
					m.ExecutionResult.ID())
			default:
				t.Logf("other msg received from %s: %#v", sender, msg)
				continue
			}
		}
	}()
}
