package lib

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/ghost/client"
	"github.com/onflow/flow-go/model/flow"
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
			case *flow.Proposal:
				err = tst.BlockState.Add(t, m)
				require.NoError(t, err)
			case *flow.ResultApproval:
				tst.ApprovalState.Add(sender, m)

			case *flow.ExecutionReceipt:
				tst.ReceiptState.Add(m)
			case *flow.ChunkDataResponse:
			default:
				continue
			}
		}
	}()
}
