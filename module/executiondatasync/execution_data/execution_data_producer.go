package execution_data

import (
	"errors"

	"go.uber.org/atomic"
)

// ErrMultipleRegister is an error returned when a producer is registered more than once.
var ErrMultipleRegister = errors.New("producer may only be registered once")

// ExecutionDataProducer is an interface for managing execution data
// producers that can be registered, which handle block processing, and provide
// the highest complete height.
type ExecutionDataProducer interface {
	// Register registers the execution data producer if it hasn't been registered already.
	// This method should only be called once for producer.
	//
	// Expected errors during normal operations:
	//   - ErrMultipleRegister: in case if producer is already registered.
	Register() error
	// OnBlockProcessed updates the highest processed height by the producer when a block is processed.
	OnBlockProcessed(uint64)
	// HighestCompleteHeight returns the highest complete block height processed by the producer.
	HighestCompleteHeight() uint64
}

var _ ExecutionDataProducer = (*ExecutionDataProducerManager)(nil)

// ExecutionDataProducerManager manages the state of an execution data producer,
// ensuring it is registered only once and tracks the highest processed block height.
type ExecutionDataProducerManager struct {
	registered            *atomic.Bool
	highestCompleteHeight *atomic.Uint64
}

// NewExecutionDataProducerManager creates a new ExecutionDataProducerManager
// with the given initial height.
func NewExecutionDataProducerManager(initHeight uint64) *ExecutionDataProducerManager {
	return &ExecutionDataProducerManager{
		registered:            atomic.NewBool(false),
		highestCompleteHeight: atomic.NewUint64(initHeight),
	}
}

// Register registers the execution data producer if it hasn't been registered already.
// This method should only be called once for producer.
//
// Expected errors during normal operations:
//   - ErrMultipleRegister: in case if producer is already registered.
func (e *ExecutionDataProducerManager) Register() error {
	// Make sure we only register once. atomically check if registered is false then set it to true.
	// If it was not false, panic
	if !e.registered.CompareAndSwap(false, true) {
		return ErrMultipleRegister
	}

	return nil
}

// OnBlockProcessed updates the highest processed height by the producer when a block is processed.
func (e *ExecutionDataProducerManager) OnBlockProcessed(height uint64) {
	if e.registered.Load() {
		if height > e.highestCompleteHeight.Load() {
			e.highestCompleteHeight.Store(height)
		}
	}
}

// HighestCompleteHeight returns the highest complete block height processed by the producer.
func (e *ExecutionDataProducerManager) HighestCompleteHeight() uint64 {
	return e.highestCompleteHeight.Load()
}
