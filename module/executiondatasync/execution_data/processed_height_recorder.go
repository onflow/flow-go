package execution_data

import (
	"sync"

	"go.uber.org/atomic"
)

type HeightUpdatesConsumer func(height uint64)

// ProcessedHeightRecorder is an interface for tracking the highest execution data processed
// height when a block is processed and for providing this height.
type ProcessedHeightRecorder interface {
	// OnBlockProcessed updates the highest processed height when a block is processed.
	OnBlockProcessed(uint64)
	// HighestCompleteHeight returns the highest complete processed block height.
	HighestCompleteHeight() uint64

	// SetHeightUpdatesConsumer subscribe consumer for processed height updates.
	// Callback are called synchronously and must be non-blocking
	SetHeightUpdatesConsumer(HeightUpdatesConsumer)
}

var _ ProcessedHeightRecorder = (*ProcessedHeightRecorderManager)(nil)

// ProcessedHeightRecorderManager manages an execution data height recorder
// and tracks the highest processed block height.
type ProcessedHeightRecorderManager struct {
	lock                  sync.RWMutex
	highestCompleteHeight *atomic.Uint64
	consumer              HeightUpdatesConsumer
}

// NewProcessedHeightRecorderManager creates a new ProcessedHeightRecorderManager with the given initial height.
func NewProcessedHeightRecorderManager(initHeight uint64) *ProcessedHeightRecorderManager {
	return &ProcessedHeightRecorderManager{
		highestCompleteHeight: atomic.NewUint64(initHeight),
	}
}

// OnBlockProcessed updates the highest processed height when a block is processed.
func (e *ProcessedHeightRecorderManager) OnBlockProcessed(height uint64) {
	if height > e.highestCompleteHeight.Load() {
		e.highestCompleteHeight.Store(height)

		e.lock.RLock()
		defer e.lock.RUnlock()
		if e.consumer != nil {
			e.consumer(height)
		}
	}
}

// HighestCompleteHeight returns the highest complete processed block height.
func (e *ProcessedHeightRecorderManager) HighestCompleteHeight() uint64 {
	return e.highestCompleteHeight.Load()
}

// SetHeightUpdatesConsumer subscribe consumers for processed height updates.
func (e *ProcessedHeightRecorderManager) SetHeightUpdatesConsumer(consumer HeightUpdatesConsumer) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.consumer = consumer
}
