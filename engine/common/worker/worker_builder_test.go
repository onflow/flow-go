package worker_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/worker"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestWorkerPool_SingleEvent_SingleWorker tests the worker pool with a single worker and a single event.
// It submits an event to the worker pool and checks if the event is processed by the worker.
func TestWorkerPool_SingleEvent_SingleWorker(t *testing.T) {
	event := "test-event"

	q := queue.NewHeroStore(10, unittest.Logger(), metrics.NewNoopCollector())
	processed := make(chan struct{})

	pool := worker.NewWorkerPoolBuilder[string](
		unittest.Logger(),
		q,
		func(input string) error {
			require.Equal(t, event, event)
			close(processed)

			return nil
		}).Build()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	cm := component.NewComponentManagerBuilder().
		AddWorker(pool.WorkerLogic()).
		Build()
	cm.Start(ctx)

	unittest.RequireCloseBefore(t, cm.Ready(), 100*time.Millisecond, "could not start worker")

	require.True(t, pool.Submit(event))

	unittest.RequireCloseBefore(t, processed, 100*time.Millisecond, "event not processed")
	cancel()
	unittest.RequireCloseBefore(t, cm.Done(), 100*time.Millisecond, "could not stop worker")
}

// TestWorkerBuilder_UnhappyPaths verifies that the WorkerBuilder can handle queue overflows, duplicate submissions.
func TestWorkerBuilder_UnhappyPaths(t *testing.T) {
	size := 5

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())

	blockingChannel := make(chan struct{})
	firstEventArrived := make(chan struct{})

	pool := worker.NewWorkerPoolBuilder[string](
		unittest.Logger(),
		q,
		func(input string) error {
			close(firstEventArrived)
			// we block the consumer to make sure that the queue is eventually full.
			<-blockingChannel

			return nil
		}).Build()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	cm := component.NewComponentManagerBuilder().
		AddWorker(pool.WorkerLogic()).
		Build()
	cm.Start(ctx)

	unittest.RequireCloseBefore(t, cm.Ready(), 100*time.Millisecond, "could not start worker")

	require.True(t, pool.Submit("first-event-ever"))

	// wait for the first event to be picked by the single worker
	unittest.RequireCloseBefore(t, firstEventArrived, 100*time.Millisecond, "first event not distributed")

	// now the worker is blocked, we submit the rest of the events so that the queue is full
	for i := 0; i < size; i++ {
		event := fmt.Sprintf("test-event-%d", i)
		require.True(t, pool.Submit(event))
		// we also check that re-submitting the same event fails as duplicate event already is in the queue.
		require.False(t, pool.Submit(event))
	}

	// now the queue is full, so the next submission should fail
	require.False(t, pool.Submit("test-event"))

	close(blockingChannel)
	cancel()
	unittest.RequireCloseBefore(t, cm.Done(), 100*time.Millisecond, "could not stop worker")
}

// TestWorkerPool_TwoWorkers_ConcurrentEvents tests the WorkerPoolBuilder with multiple events and two workers.
// It submits multiple events to the WorkerPool concurrently and checks if each event is processed exactly once.
func TestWorkerPool_TwoWorkers_ConcurrentEvents(t *testing.T) {
	size := 10

	tc := make([]string, size)

	for i := 0; i < size; i++ {
		tc[i] = fmt.Sprintf("test-event-%d", i)
	}

	q := queue.NewHeroStore(uint32(size), unittest.Logger(), metrics.NewNoopCollector())
	distributedEvents := unittest.NewProtectedMap[string, struct{}]()
	allEventsDistributed := sync.WaitGroup{}
	allEventsDistributed.Add(size)

	pool := worker.NewWorkerPoolBuilder[string](
		unittest.Logger(),
		q,
		func(event string) error {
			// check if the event is in the test case
			require.Contains(t, tc, event)

			// check if the event is distributed only once
			require.False(t, distributedEvents.Has(event))
			distributedEvents.Add(event, struct{}{})

			allEventsDistributed.Done()

			return nil
		}).Build()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, _ := irrecoverable.WithSignaler(cancelCtx)
	cm := component.NewComponentManagerBuilder().
		AddWorker(pool.WorkerLogic()).
		AddWorker(pool.WorkerLogic()).
		Build()
	cm.Start(ctx)

	unittest.RequireCloseBefore(t, cm.Ready(), 100*time.Millisecond, "could not start worker")

	for i := 0; i < size; i++ {
		go func(i int) {
			require.True(t, pool.Submit(tc[i]))
		}(i)
	}

	unittest.RequireReturnsBefore(t, allEventsDistributed.Wait, 100*time.Millisecond, "events not processed")
	cancel()
	unittest.RequireCloseBefore(t, cm.Done(), 100*time.Millisecond, "could not stop worker")
}
