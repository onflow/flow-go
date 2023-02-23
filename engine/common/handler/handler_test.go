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
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAsyncEventHandler_SingleEvent tests the async event handler with a single event. It submits an event to the handler
// and checks if the event is processed by the handler.
func TestAsyncEventHandler_SingleEvent(t *testing.T) {
	event := "test-event"

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	eventProcessed := make(chan struct{})
	h := handler.NewAsyncEventHandler(unittest.Logger(), q, func(originId flow.Identifier, event interface{}) {
		require.Equal(t, originId, originId)
		require.Equal(t, event, event)
		close(eventProcessed)
	}, 2)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	require.NoError(t, h.Submit(event))

	unittest.RequireCloseBefore(t, eventProcessed, 100*time.Millisecond, "event not processed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}

// TestAsyncEventHandler_SubmissionError tests the async event handler checks that the submission of an event fails if
// the queue is full or the event is duplicate in the queue. It also checks that when the submission fails, the handler
// returns an ErrSubmissionFailed error.
func TestAsyncEventHandler_SubmissionError(t *testing.T) {
	size := 5

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())

	blockingChannel := make(chan struct{})
	h := handler.NewAsyncEventHandler(unittest.Logger(), q, func(originId flow.Identifier, event interface{}) {
		// we block the processor to make sure that the queue is eventually full.
		<-blockingChannel
	}, 1)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	for i := 0; i < size+1; i++ {
		event := fmt.Sprintf("test-event-%d", i)
		require.NoError(t, h.Submit(event), i)
		require.ErrorIs(t, h.Submit(event), handler.ErrSubmissionFailed) // duplicate event should be rejected
	}
	// the queue is full, so the next submission should fail
	require.ErrorIs(t, h.Submit("test-event"), handler.ErrSubmissionFailed)

	close(blockingChannel)
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}

// TestAsyncEventHandler_MultipleConcurrentEvents_DistinctIdentifier tests the async event handler with multiple events
// with distinct origin identifiers. It submits multiple events to the handler and checks if each event is processed by
// the handler exactly once.
func TestAsyncEventHandler_MultipleConcurrentEvents_DistinctIdentifier(t *testing.T) {
	size := 10
	workers := uint(5)

	tc := make([]struct {
		event interface{}
	}, 10)

	for i := 0; i < size; i++ {
		tc[i].event = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())
	processedEvents := unittest.NewProtectedMap[string, struct{}]()
	allEventsProcessed := sync.WaitGroup{}
	allEventsProcessed.Add(size)

	h := handler.NewAsyncEventHandler(unittest.Logger(), q, func(originId flow.Identifier, event interface{}) {
		// check if the event is in the test case
		require.Contains(t, tc, struct {
			event interface{}
		}{
			event: event,
		})

		// check if the event is processed only once
		require.False(t, processedEvents.Has(event.(string)))
		processedEvents.Add(event.(string), struct{}{})

		allEventsProcessed.Done()
	}, workers)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	for i := 0; i < size; i++ {
		go func(i int) {
			require.NoError(t, h.Submit(tc[i].event))
		}(i)
	}

	unittest.RequireReturnsBefore(t, allEventsProcessed.Wait, 10*time.Second, "events not processed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}
