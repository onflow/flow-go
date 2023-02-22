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
	originId := unittest.IdentifierFixture()
	event := "test-event"

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	h := handler.NewAsyncEventHandler(unittest.Logger(), q, 2)
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

// testAsyncEventHandlerWithMultipleConcurrentEvents is a test helper that tests the async event handler with multiple events.
// It submits multiple events to the handler and checks if each event is processed by the handler exactly once.
// The second argument is a function that generates a new origin identifier for each event.
func testAsyncEventHandlerWithMultipleConcurrentEvents(t *testing.T, idFactory func() flow.Identifier) {
	size := 10
	workers := uint(5)

	tc := make([]struct {
		originId flow.Identifier
		event    interface{}
	}, 10)

	for i := 0; i < size; i++ {
		tc[i].originId = idFactory()
		tc[i].event = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())
	h := handler.NewAsyncEventHandler(unittest.Logger(), q, workers)

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

// TestAsyncEventHandler_MultipleConcurrentEvents_DistinctIdentifier tests the async event handler with multiple events
// with distinct origin identifiers. It submits multiple events to the handler and checks if each event is processed by
// the handler exactly once.
func TestAsyncEventHandler_MultipleConcurrentEvents_DistinctIdentifier(t *testing.T) {
	testAsyncEventHandlerWithMultipleConcurrentEvents(t, func() flow.Identifier {
		return unittest.IdentifierFixture()
	})
}

// TestAsyncEventHandler_MultipleConcurrentEvents_IdenticalIdentifier tests the async event handler with multiple events
// with identical origin identifiers. It submits multiple events to the handler and checks if each event is processed by
// the handler exactly once.
// This test is to ensure that the async event handler does not rely on the origin identifier for processing events and
// can process events with identical origin identifiers concurrently without any issues. In other words, the async event
// handler should be able to process events concurrently even if the origin identifier is not unique.
func TestAsyncEventHandler_MultipleConcurrentEvents_IdenticalIdentifier(t *testing.T) {
	testAsyncEventHandlerWithMultipleConcurrentEvents(t, func() flow.Identifier {
		return flow.ZeroID
	})
}
