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

// mockConsumer is a mock implementation of the EventConsumer interface. It is used to test the AsyncEventDistributor.
type mockConsumer struct {
	consumeFn func(event string)
}

func (p *mockConsumer) ConsumeEvent(event string) {
	p.consumeFn(event)
}

// TestAsyncEventDistributor_SingleEvent_SingleConsumer tests the AsyncEventDistributor with a single event.
// It submits an event to the distributor and checks if the event is distributed by the distributor.
func TestAsyncEventDistributor_SingleEvent_SingleConsumer(t *testing.T) {
	event := "test-event"

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	eventDistributed := make(chan struct{})
	cosumer := &mockConsumer{
		consumeFn: func(event string) {
			require.Equal(t, event, event)
			close(eventDistributed)
		},
	}

	h := handler.NewAsyncEventDistributor[string](unittest.Logger(), q, 2)
	h.RegisterConsumer(cosumer)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start distributor")

	require.NoError(t, h.Submit(event))

	unittest.RequireCloseBefore(t, eventDistributed, 100*time.Millisecond, "event not distributed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop distributor")
}

// TestAsyncEventDistributor_SubmissionError tests the AsyncEventDistributor checks that the submission of an event fails if
// the queue is full or the event is duplicate in the queue. It also checks that when the submission fails, the distributor
// returns an ErrSubmissionFailed error.
func TestAsyncEventDistributor_SubmissionError(t *testing.T) {
	size := 5

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())

	blockingChannel := make(chan struct{})
	firstEventArrived := make(chan struct{})
	consumer := &mockConsumer{
		consumeFn: func(event string) {
			close(firstEventArrived)
			// we block the consumer to make sure that the queue is eventually full.
			<-blockingChannel
		},
	}
	h := handler.NewAsyncEventDistributor[string](unittest.Logger(), q, 1)
	h.RegisterConsumer(consumer)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start distributor")

	// first event should be submitted successfully
	require.NoError(t, h.Submit("first-event-ever"))

	// wait for the first event to be picked by the single worker
	unittest.RequireCloseBefore(t, firstEventArrived, 100*time.Millisecond, "first event not distributed")

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

// TestAsyncEventDistributor_MultipleConcurrentEvents tests the AsyncEventDistributor with multiple events.
// It submits multiple events to the distributor and checks if each event is distributed exactly once.
func TestAsyncEventDistributor_MultipleConcurrentEvents(t *testing.T) {
	size := 10
	workers := uint(5)

	tc := make([]string, size)

	for i := 0; i < size; i++ {
		tc[i] = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())
	distributedEvents := unittest.NewProtectedMap[string, struct{}]()
	allEventsDistributed := sync.WaitGroup{}
	allEventsDistributed.Add(size)

	distributor := &mockConsumer{
		consumeFn: func(event string) {
			// check if the event is in the test case
			require.Contains(t, tc, event)

			// check if the event is distributed only once
			require.False(t, distributedEvents.Has(event))
			distributedEvents.Add(event, struct{}{})

			allEventsDistributed.Done()
		},
	}
	h := handler.NewAsyncEventDistributor[string](unittest.Logger(), q, workers)
	h.RegisterConsumer(distributor)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start distributor")

	for i := 0; i < size; i++ {
		go func(i int) {
			require.NoError(t, h.Submit(tc[i]))
		}(i)
	}

	unittest.RequireReturnsBefore(t, allEventsDistributed.Wait, 10*time.Second, "events not distributed")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop distributor")
}

// TestAsyncEventDistributor_MultipleEvents_MultipleConsumers tests the AsyncEventDistributor with multiple events and multiple consumer.
// It submits multiple events to the distributor and checks if each event is distributed exactly once to all consumers.
func TestNewAsyncEventDistributor_MultipleEvents_MultipleConsumers(t *testing.T) {
	events := make([]string, 10)
	for i := 0; i < 10; i++ {
		events[i] = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	c1EventSeen := unittest.NewProtectedMap[string, struct{}]()
	c1Done := sync.WaitGroup{}
	c1Done.Add(len(events))
	consumer1 := &mockConsumer{
		consumeFn: func(event string) {
			// check if the event is in the test case
			require.Contains(t, events, event)

			// check if the event is distributed only once
			require.False(t, c1EventSeen.Has(event))
			c1EventSeen.Add(event, struct{}{})

			c1Done.Done()
		},
	}

	c2EventSeen := unittest.NewProtectedMap[string, struct{}]()
	c2Done := sync.WaitGroup{}
	c2Done.Add(len(events))
	consumer2 := &mockConsumer{
		consumeFn: func(event string) {
			// check if the event is in the test case
			require.Contains(t, events, event)

			// check if the event is distributed only once
			require.False(t, c2EventSeen.Has(event))
			c2EventSeen.Add(event, struct{}{})

			c2Done.Done()
		},
	}

	h := handler.NewAsyncEventDistributor[string](unittest.Logger(), q, 2)
	h.RegisterConsumer(consumer1)
	h.RegisterConsumer(consumer2)

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	h.Start(ctx)

	unittest.RequireCloseBefore(t, h.Ready(), 100*time.Millisecond, "could not start distributor")

	for _, event := range events {
		require.NoError(t, h.Submit(event))
	}

	unittest.RequireReturnsBefore(t, c1Done.Wait, 100*time.Millisecond, "consumer 1 did not consume all events")
	unittest.RequireReturnsBefore(t, c2Done.Wait, 100*time.Millisecond, "consumer 2 did not consume all events")
	cancel()
	unittest.RequireCloseBefore(t, h.Done(), 100*time.Millisecond, "could not stop distributor")
}
