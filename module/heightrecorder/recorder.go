package heightrecorder

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/counters"
)

// ProcessedHeightRecorder is an interface for tracking the highest height processed by a component.
type ProcessedHeightRecorder interface {
	// OnBlockProcessed updates the highest processed height when a block is processed.
	OnBlockProcessed(uint64)

	// HighestCompleteHeight returns the highest complete processed block height.
	// If no height has been processed, false is returned.
	HighestCompleteHeight() (uint64, bool)
}

var _ ProcessedHeightRecorder = (*processedHeightRecorderImpl)(nil)

type processedHeightRecorderImpl struct {
	highestCompleteHeight counters.StrictMonotonousCounter
	initialized           *atomic.Bool
}

// NewProcessedHeightRecorder creates a new ProcessedHeightRecorder implementation with the given
// initial height.
func NewProcessedHeightRecorder() *processedHeightRecorderImpl {
	return &processedHeightRecorderImpl{
		highestCompleteHeight: counters.NewMonotonousCounter(0),
		initialized:           atomic.NewBool(false),
	}
}

// OnBlockProcessed updates the highest processed height when a block is processed.
func (e *processedHeightRecorderImpl) OnBlockProcessed(newValue uint64) {
	e.highestCompleteHeight.Set(newValue)
	if !e.initialized.Load() {
		e.initialized.Store(true)
	}
}

// HighestCompleteHeight returns the highest complete processed block height.
// If no height has been processed, false is returned.
func (e *processedHeightRecorderImpl) HighestCompleteHeight() (uint64, bool) {
	if !e.initialized.Load() {
		return 0, false
	}
	return e.highestCompleteHeight.Value(), true
}
