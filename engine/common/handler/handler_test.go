package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/handler"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAsyncEventHandler_SingleEvent tests the async event handler with a single event. It submits an event to the handler
// and checks if the event is processed by the handler.
func TestAsyncEventHandler_SingleEvent(t *testing.T) {
	originId := unittest.IdentifierFixture()
	event := "test-event"

	// Test case 1: Test submit method
	h := handler.NewAsyncEventHandler(unittest.Logger(), 10, 2)
	eventProcessed := make(chan struct{})
	h.RegisterProcessor(func(originId flow.Identifier, event interface{}) {
		require.Equal(t, originId, originId)
		require.Equal(t, event, event)
		close(eventProcessed)
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	require.NoError(t, h.Submit(originId, event))

	unittest.RequireCloseBefore(t, eventProcessed, 100*time.Millisecond, "event not processed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}
