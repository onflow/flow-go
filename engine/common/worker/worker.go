package worker

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
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

// NewWorkerLogic returns a new worker logic that can be added to a component. The worker logic is responsible for
// processing the work that is submitted to the worker pool.
// A pool may have multiple workers, but the worker logic is the same for all the workers.
// Workers are managed by the higher-level component, through component.AddWorker.
func (p *Pool[T]) NewWorkerLogic() component.ComponentWorker {
	return p.workerLogic
}

// SubmitLogic returns the logic that the higher-level component executes to submit work to the worker pool.
// The submit logic is responsible for submitting the work to the worker pool.
func (p *Pool[T]) SubmitLogic() func(event T) bool {
	return p.submitLogic
}

// PoolBuilder is an auxiliary builder for constructing workers with a common inbound queue,
// where the workers are managed by a higher-level component. The message store as well as the processing
// function are specified by the caller.
// WorkerPoolBuilder does not add any concurrency handling. It is the callers responsibility to make sure
// that the number of workers concurrently accessing `processingFunc` is compatible with its implementation.
type PoolBuilder[T any] struct {
	store    engine.MessageStore // temporarily store inbound events till they are processed.
	notifier engine.Notifier

	// processingFunc is the function for processing the input tasks. It should only return unexpected
	// exceptions. Sentinel errors expected during normal operations should be handled internally.
	processingFunc func(T) error
}

// NewWorkerPoolBuilder instantiates a new WorkerPoolBuilder
func NewWorkerPoolBuilder[T any](
	store engine.MessageStore,
	processingFunc func(input T) error,
) *PoolBuilder[T] {
	return &PoolBuilder[T]{
		store:          store,
		notifier:       engine.NewNotifier(),
		processingFunc: processingFunc,
	}
}

// Build creates
//   - [first return value] the logic that the worker executes (can be added as multiple
//     times to Component via Component.AddWorker or Component.AddWorkers as long as
//     `processingFunc` is concurrency safe)
//   - [second return value] the logic for submitting work to the message store. This
//     function yields true, if work was successfully submitted and false otherwise.
func (b *PoolBuilder[T]) Build() *Pool[T] {
	return &Pool[T]{
		workerLogic: b.workerLogic(),
		submitLogic: b.submitLogic(),
	}
}

// workerLogic return the worker logic itself
func (b *PoolBuilder[T]) workerLogic() component.ComponentWorker {
	notifier := b.notifier.Channel()
	processingFunc := b.processingFunc
	store := b.store

	return func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		ready() // wait for ready signal

		for {
			select {
			case <-ctx.Done():
				return
			case <-notifier:
				for { // on single notification, commence processing items in store until none left
					select {
					case <-ctx.Done():
						return
					default:
					}

					msg, ok := store.Get()
					if !ok {
						break // store is empty; go back to outer for loop
					}
					err := processingFunc(msg.Payload.(T))
					if err != nil {
						ctx.Throw(fmt.Errorf("unexpected error processing queued work item: %w", err))
						return
					}
				}
			}
		}
	}
}

// submitLogic workerLogic return an abstract function for submitting work to the message store.
// The returned function yields true, if work was successfully submitted and false otherwise.
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
