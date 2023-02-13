package handler_test

import (
	"context"
	"fmt"
	"sync"
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

// TestAsyncEventHandler_MultipleEvents tests the async event handler with multiple events. It submits multiple events to
// the handler and checks if the events are processed by the handler.
func TestAsyncEventHandler_MultipleEvents(t *testing.T) {
	size := 10
	workers := uint(5)

	tc := make([]struct {
		originId flow.Identifier
		event    interface{}
	}, 10)

	for i := 0; i < size; i++ {
		tc[i].originId = unittest.IdentifierFixture()
		tc[i].event = fmt.Sprintf("test-event-%d", i)
	}

	h := handler.NewAsyncEventHandler(unittest.Logger(), uint32(size), workers)

	processedEvents := unittest.NewProtectedMap[string, struct{}]()
	allEventsProcessed := sync.WaitGroup{}
	allEventsProcessed.Add(size)
	h.RegisterProcessor(func(originId flow.Identifier, event interface{}) {
		// check if the event is in the test case
		require.Contains(t, tc, struct {
			originId flow.Identifier
			event    interface{}
		}{
			originId: originId,
			event:    event,
		})

		// check if the event is processed only once
		require.False(t, processedEvents.Has(event.(string)))
		processedEvents.Add(event.(string), struct{}{})

		allEventsProcessed.Done()
	})

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	for i := 0; i < size; i++ {
		go func(i int) {
			require.NoError(t, h.Submit(tc[i].originId, tc[i].event))
		}(i)
	}

	unittest.RequireReturnsBefore(t, allEventsProcessed.Wait, 10*time.Second, "events not processed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}
