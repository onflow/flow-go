package execution_data

import (
	"errors"

	"go.uber.org/atomic"
)

var ErrMultipleRegister = errors.New("producer may only be registered once")

type ExecutionDataProducer interface {
	Register() error
	OnBlockProcessed(uint64)
	HighestCompleteHeight() uint64
}

var _ ExecutionDataProducer = (*ExecutionDataProducerManager)(nil)

type ExecutionDataProducerManager struct {
	registered          *atomic.Bool
	lastProcessedHeight *atomic.Uint64
}

func NewExecutionDataProducerManager(initHeight uint64) *ExecutionDataProducerManager {
	return &ExecutionDataProducerManager{
		registered:          atomic.NewBool(false),
		lastProcessedHeight: atomic.NewUint64(initHeight),
	}
}

func (e *ExecutionDataProducerManager) Register() error {
	// Make sure we only register once. atomically check if registered is false then set it to true.
	// If it was not false, panic
	if !e.registered.CompareAndSwap(false, true) {
		return ErrMultipleRegister
	}

	return nil
}

func (e *ExecutionDataProducerManager) OnBlockProcessed(height uint64) {
	if e.registered.Load() {
		e.lastProcessedHeight.Store(height)
	}
}

func (e *ExecutionDataProducerManager) HighestCompleteHeight() uint64 {
	return e.lastProcessedHeight.Load()
}
