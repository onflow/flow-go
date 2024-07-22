package execution_data

import (
	"errors"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
)

var ErrMultipleRegister = errors.New("producer may only be registered once")

type ExecutionDataProducer interface {
	Register(*engine.Notifier) error
	OnBlockProcessed(uint64)
	HighestCompleteHeight() uint64
}

var _ ExecutionDataProducer = (*ExecutionDataProducerManager)(nil)

type ExecutionDataProducerManager struct {
	registered          *atomic.Bool
	producerNotifier    *engine.Notifier
	lastProcessedHeight *atomic.Uint64
}

func NewExecutionDataProducerManager(initHeight uint64) *ExecutionDataProducerManager {
	return &ExecutionDataProducerManager{
		registered:          atomic.NewBool(false),
		lastProcessedHeight: atomic.NewUint64(initHeight),
		producerNotifier:    nil,
	}
}

func (e *ExecutionDataProducerManager) Register(notifier *engine.Notifier) error {
	// Make sure we only register once. atomically check if registered is false then set it to true.
	// If it was not false, panic
	if !e.registered.CompareAndSwap(false, true) {
		return ErrMultipleRegister
	}
	e.producerNotifier = notifier

	return nil
}

func (e *ExecutionDataProducerManager) OnBlockProcessed(height uint64) {
	e.lastProcessedHeight.Store(height)
	if e.registered.Load() {
		e.producerNotifier.Notify()
	}
}

func (e *ExecutionDataProducerManager) HighestCompleteHeight() uint64 {
	return e.lastProcessedHeight.Load()
}
