package common

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
)

type TestnetStateTracker struct {
	ghostTracking bool
	BlockState    *BlockState
	ReceiptState  ReceiptState
	ApprovalState ResultApprovalState
	MsgState      MsgState
}

// Track starts to track new blocks, execution receipts and individual messages the ghost receives. The context will
// be used to stop tracking
func (tst *TestnetStateTracker) Track(t *testing.T, ctx context.Context, ghost *client.GhostClient) {
	// reset the state for in between tests
	tst.BlockState = NewBlockState()
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
		count := uint64(0)
		for {
			count++
			fmt.Printf("re-started reader loop: %d \n", count)
			sender, msg, err := reader.Next()
			fmt.Printf("read next message: %d \n", count)

			select {
			// don't error if container shuts down
			case <-ctx.Done():
				fmt.Printf("context done: %d \n", count)
				return
			default:
				// continue with this iteration of the loop
			}

			if err != nil && strings.Contains(err.Error(), "transport is closing") {
				fmt.Printf("error occured: %d error: %w \n", count, err)
				return
			}

			// don't allow other errors
			require.NoError(t, err, "could not read next message")

			tst.MsgState.Add(sender, msg)
			fmt.Printf("message added to message state: %d \n", count)

			switch m := msg.(type) {
			case *messages.BlockProposal:
				start := time.Now()
				fmt.Printf("adding message to block state: %d \n", count)
				tst.BlockState.Add(m)
				fmt.Printf("added message to block state: %d \n", count)
				t.Logf("block proposal received from %s at height %v: %x, took: %d",
					sender,
					m.Header.Height,
					m.Header.ID(), time.Since(start).Milliseconds())
			case *flow.ResultApproval:
				fmt.Printf("adding message to result approval: %d \n", count)
				tst.ApprovalState.Add(sender, m)
				fmt.Printf("added message to result approval: %d \n", count)
				t.Logf("result approval received from %s for execution result ID %x and chunk index %v",
					sender,
					m.Body.ExecutionResultID,
					m.Body.ChunkIndex)
			case *flow.ExecutionReceipt:
				fmt.Printf("adding message to receipt state: %d \n", count)
				finalState, err := m.ExecutionResult.FinalStateCommitment()
				require.NoError(t, err)

				tst.ReceiptState.Add(m)
				fmt.Printf("added message to receipt state: %d \n", count)
				t.Logf("execution receipts received from %s for block ID %x by executor ID %x with SC %x resultID %x",
					sender,
					m.ExecutionResult.BlockID,
					m.ExecutorID,
					finalState,
					m.ExecutionResult.ID())
			default:
				fmt.Printf("other message, continuing loop: %d \n", count)
				t.Logf("other msg received from %s: %#v", sender, msg)
				continue
			}
		}
	}()
}
