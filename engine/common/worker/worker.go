package worker

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

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
func (b *PoolBuilder[T]) Build() (component.ComponentWorker, func(event T) bool) {
	return b.workerLogic(), b.submitLogic()
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
