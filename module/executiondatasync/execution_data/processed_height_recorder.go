package execution_data

import (
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

	// AddHeightUpdatesConsumer subscribe consumers for processed height updates.
	AddHeightUpdatesConsumer(HeightUpdatesConsumer)
}

var _ ProcessedHeightRecorder = (*ProcessedHeightRecorderManager)(nil)

// ProcessedHeightRecorderManager manages an execution data height recorder
// and tracks the highest processed block height.
type ProcessedHeightRecorderManager struct {
	highestCompleteHeight *atomic.Uint64
	consumers             []HeightUpdatesConsumer
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

		for _, consumer := range e.consumers {
			consumer(height)
		}
	}
}

// HighestCompleteHeight returns the highest complete processed block height.
func (e *ProcessedHeightRecorderManager) HighestCompleteHeight() uint64 {
	return e.highestCompleteHeight.Load()
}

// AddHeightUpdatesConsumer subscribe consumers for processed height updates.
func (e *ProcessedHeightRecorderManager) AddHeightUpdatesConsumer(consumer HeightUpdatesConsumer) {
	e.consumers = append(e.consumers, consumer)
}
