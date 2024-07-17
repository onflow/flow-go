package execution_data

import (
	"errors"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
)

var ErrMultipleRegister = errors.New("producer may only be registered once")

type ExecutionDataProducer interface {
	Register(*engine.Notifier)
	SetLastProcessedHeight(uint64)
	LastProcessedHeight() uint64
}

var _ ExecutionDataProducer = (*ExecutionDataProducerManager)(nil)

type ExecutionDataProducerManager struct {
	registered          *atomic.Bool
	producerNotifier    *engine.Notifier
	lastProcessedHeight *atomic.Uint64
}

func NewExecutionDataProducerManager() *ExecutionDataProducerManager {
	return &ExecutionDataProducerManager{
		registered:          atomic.NewBool(false),
		lastProcessedHeight: atomic.NewUint64(0),
		producerNotifier:    nil,
	}
}

func (e *ExecutionDataProducerManager) Register(notifier *engine.Notifier) {
	// Make sure we only register once. atomically check if registered is false then set it to true.
	// If it was not false, panic
	if !e.registered.CompareAndSwap(false, true) {
		panic(ErrMultipleRegister)
	}
	e.producerNotifier = notifier
}

func (e *ExecutionDataProducerManager) SetLastProcessedHeight(height uint64) {
	e.lastProcessedHeight.Store(height)
	if e.registered.Load() {
		e.producerNotifier.Notify()
	}
}

func (e *ExecutionDataProducerManager) LastProcessedHeight() uint64 {
	return e.lastProcessedHeight.Load()
}
