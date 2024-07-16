package execution_data

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module"
)

type ExecutionDataProducer interface {
	Register(*engine.Notifier)
	LastProducedHeight() (uint64, error)
	NotifyProducedHeight()
}

type ExecutionDataProducerManager struct {
	registered       *atomic.Bool
	producerNotifier *engine.Notifier
}

func NewExecutionDataProducerManager() *ExecutionDataProducerManager {
	return &ExecutionDataProducerManager{
		registered:       atomic.NewBool(false),
		producerNotifier: nil,
	}
}

func (e *ExecutionDataProducerManager) Register(notifier *engine.Notifier) {
	// Make sure we only register once. atomically check if registered is false then set it to true.
	// If it was not false, panic
	if !e.registered.CompareAndSwap(false, true) {
		panic(module.ErrMultipleStartup)
	}
	e.producerNotifier = notifier
}

func (e *ExecutionDataProducerManager) NotifyProducedHeight() {
	if e.registered.Load() {
		e.producerNotifier.Notify()
	}
}
