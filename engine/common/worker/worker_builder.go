package worker

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

const (
	QueuedItemProcessingLog = "processing queued work item"
	QueuedItemProcessedLog  = "finished processing queued work item"
)

// Pool is a worker pool that can be used by a higher-level component to manage a set of workers.
// The workers are managed by the higher-level component, but the worker pool provides the logic for
// submitting work to the workers and for processing the work. The worker pool is responsible for
// storing the work until it is processed by a worker.
type Pool[T any] struct {
	// workerLogic is the logic that the worker executes. It is the responsibility of the higher-level
	// component to add this logic to the component. The worker logic is responsible for processing
	// the work that is submitted to the worker pool.
	// The worker logic should only throw unexpected exceptions to its component. Sentinel errors expected during
	// normal operations should be handled internally.
	// The worker logic should not throw any exceptions that are not expected during normal operations.
	// Any exceptions thrown by the worker logic will be caught by the higher-level component and will cause the
	// component to crash.
	// A pool may have multiple workers, but the worker logic is the same for all the workers.
	workerLogic component.ComponentWorker

	// submitLogic is the logic that the higher-level component executes to submit work to the worker pool.
	// The submit logic is responsible for submitting the work to the worker pool. The submit logic should
	// is responsible for storing the work until it is processed by a worker. The submit should handle
	// any errors that occur during the submission of the work internally.
	// The return value of the submit logic indicates whether the work was successfully submitted to the worker pool.
	submitLogic func(event T) bool
}

// WorkerLogic returns a new worker logic that can be added to a component. The worker logic is responsible for
// processing the work that is submitted to the worker pool.
// A pool may have multiple workers, but the worker logic is the same for all the workers.
// Workers are managed by the higher-level component, through component.AddWorker.
func (p *Pool[T]) WorkerLogic() component.ComponentWorker {
	return p.workerLogic
}

// Submit submits work to the worker pool. The submit logic is responsible for submitting the work to the worker pool.
func (p *Pool[T]) Submit(event T) bool {
	return p.submitLogic(event)
}

// PoolBuilder is an auxiliary builder for constructing workers with a common inbound queue,
// where the workers are managed by a higher-level component.
//
// The message store as well as the processing function are specified by the caller.
// WorkerPoolBuilder does not add any concurrency handling.
// It is the callers responsibility to make sure that the number of workers concurrently accessing `processingFunc`
// is compatible with its implementation.
type PoolBuilder[T any] struct {
	logger   zerolog.Logger
	store    engine.MessageStore // temporarily store inbound events till they are processed.
	notifier engine.Notifier

	// processingFunc is the function for processing the input tasks. It should only return unexpected
	// exceptions. Sentinel errors expected during normal operations should be handled internally.
	processingFunc func(T) error
}

// NewWorkerPoolBuilder creates a new PoolBuilder, which is an auxiliary builder
// for constructing workers with a common inbound queue.
// Arguments:
// -`processingFunc`: the function for processing the input tasks.
// -`store`: temporarily stores inbound events until they are processed.
// Returns:
// The function returns a `PoolBuilder` instance.
func NewWorkerPoolBuilder[T any](
	logger zerolog.Logger,
	store engine.MessageStore,
	processingFunc func(input T) error,
) *PoolBuilder[T] {
	return &PoolBuilder[T]{
		logger:         logger.With().Str("component", "worker-pool").Logger(),
		store:          store,
		notifier:       engine.NewNotifier(),
		processingFunc: processingFunc,
	}
}

// Build builds a new worker pool. The worker pool is responsible for storing the work until it is processed by a worker.
func (b *PoolBuilder[T]) Build() *Pool[T] {
	return &Pool[T]{
		workerLogic: b.workerLogic(),
		submitLogic: b.submitLogic(),
	}
}

// workerLogic returns an abstract function for processing work from the message store.
// The worker logic picks up work from the message store and processes it.
// The worker logic should only throw unexpected exceptions to its component. Sentinel errors expected during
// normal operations should be handled internally.
// The worker logic should not throw any exceptions that are not expected during normal operations.
// Any exceptions thrown by the worker logic will be caught by the higher-level component and will cause the
// component to crash.
func (b *PoolBuilder[T]) workerLogic() component.ComponentWorker {
	notifier := b.notifier.Channel()
	processingFunc := b.processingFunc
	store := b.store

	return func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready() // wait for ready signal

		for {
			select {
			case <-ctx.Done():
				b.logger.Trace().Msg("worker logic shutting down")
				return
			case <-notifier:
				for { // on single notification, commence processing items in store until none left
					select {
					case <-ctx.Done():
						b.logger.Trace().Msg("worker logic shutting down")
						return
					default:
					}

					msg, ok := store.Get()
					if !ok {
						b.logger.Trace().Msg("store is empty, waiting for next notification")
						break // store is empty; go back to outer for loop
					}
					b.logger.Trace().Msg(QueuedItemProcessingLog)
					err := processingFunc(msg.Payload.(T))
					b.logger.Trace().Msg(QueuedItemProcessedLog)
					if err != nil {
						ctx.Throw(fmt.Errorf("unexpected error processing queued work item: %w", err))
						return
					}
				}
			}
		}
	}
}

// submitLogic returns an abstract function for submitting work to the message store.
// The submit logic is responsible for submitting the work to the worker pool. The submit logic should
// is responsible for storing the work until it is processed by a worker. The submit should handle
// any errors that occur during the submission of the work internally.
func (b *PoolBuilder[T]) submitLogic() func(event T) bool {
	store := b.store

	return func(event T) bool {
		ok := store.Put(&engine.Message{
			Payload: event,
		})
		if !ok {
			return false
		}
		b.notifier.Notify()
		return true
	}
}
