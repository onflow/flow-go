package handler_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/handler"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type mockProcessor struct {
	processFn func(event string)
}

func (p *mockProcessor) ProcessEvent(event string) {
	p.processFn(event)
}

// TestAsyncEventHandler_SingleEvent tests the async event handler with a single event. It submits an event to the handler
// and checks if the event is processed by the handler.
func TestAsyncEventHandler_SingleEvent(t *testing.T) {
	event := "test-event"

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	eventProcessed := make(chan struct{})
	processor := &mockProcessor{
		processFn: func(event string) {
			require.Equal(t, event, event)
			close(eventProcessed)
		},
	}

	h := handler.NewAsyncEventHandler[string](unittest.Logger(), q, 2)
	h.RegisterProcessor(processor)

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
	firstEventArrived := make(chan struct{})
	processor := &mockProcessor{
		processFn: func(event string) {
			close(firstEventArrived)
			// we block the processor to make sure that the queue is eventually full.
			<-blockingChannel
		},
	}
	h := handler.NewAsyncEventHandler[string](unittest.Logger(), q, 1)
	h.RegisterProcessor(processor)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	// first event should be submitted successfully
	require.NoError(t, h.Submit("first-event-ever"))

	// wait for the first event to be picked by the single worker
	unittest.RequireCloseBefore(t, firstEventArrived, 100*time.Millisecond, "first event not processed")

	// now the worker is blocked, we submit the rest of the events so that the queue is full
	for i := 0; i < size; i++ {
		event := fmt.Sprintf("test-event-%d", i)
		require.NoError(t, h.Submit(event))
		// we also check that re-submitting the same event fails as duplicate event already is in the queue
		require.ErrorIs(t, h.Submit(event), handler.ErrSubmissionFailed)
	}

	// now the queue is full, so the next submission should fail
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

	tc := make([]string, size)

	for i := 0; i < size; i++ {
		tc[i] = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())
	processedEvents := unittest.NewProtectedMap[string, struct{}]()
	allEventsProcessed := sync.WaitGroup{}
	allEventsProcessed.Add(size)

	processor := &mockProcessor{
		processFn: func(event string) {
			// check if the event is in the test case
			require.Contains(t, tc, event)

			// check if the event is processed only once
			require.False(t, processedEvents.Has(event))
			processedEvents.Add(event, struct{}{})

			allEventsProcessed.Done()
		},
	}
	h := handler.NewAsyncEventHandler[string](unittest.Logger(), q, workers)
	h.RegisterProcessor(processor)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start handler")

	for i := 0; i < size; i++ {
		go func(i int) {
			require.NoError(t, h.Submit(tc[i]))
		}(i)
	}

	unittest.RequireReturnsBefore(t, allEventsProcessed.Wait, 10*time.Second, "events not processed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop handler")
}
