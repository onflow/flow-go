package collections

import (
	"github.com/onflow/flow-go/engine/access/ingestion2"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

var _ ingestion2.EDIHeightProvider = (*ProcessedHeightRecorderWrapper)(nil)

// ProcessedHeightRecorderWrapper wraps execution_data.ProcessedHeightRecorder to implement
// ingestion2.EDIHeightProvider interface.
type ProcessedHeightRecorderWrapper struct {
	recorder execution_data.ProcessedHeightRecorder
}

// NewProcessedHeightRecorderWrapper creates a new wrapper that implements EDIHeightProvider
// by wrapping the given ProcessedHeightRecorder.
func NewProcessedHeightRecorderWrapper(recorder execution_data.ProcessedHeightRecorder) *ProcessedHeightRecorderWrapper {
	return &ProcessedHeightRecorderWrapper{
		recorder: recorder,
	}
}

// HighestIndexedHeight returns the highest block height for which EDI has indexed collections.
// It wraps the ProcessedHeightRecorder's HighestCompleteHeight method.
func (p *ProcessedHeightRecorderWrapper) HighestIndexedHeight() (uint64, error) {
	return p.recorder.HighestCompleteHeight(), nil
}

