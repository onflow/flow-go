package blockproducer

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// BlockBuilderMetricsWrapper implements the module.Builder interface.
// It wraps a module.Builder instance and measures the time which HotStuff's core logic
// spends in the module.Builder component, i.e. the with generating block payloads.
// The measured time durations are reported as values for the
// PayloadProductionDuration metric.
type BlockBuilderMetricsWrapper struct {
	builder module.Builder
	metrics module.HotstuffMetrics
}

func NewMetricsWrapper(builder module.Builder, metrics module.HotstuffMetrics) *BlockBuilderMetricsWrapper {
	return &BlockBuilderMetricsWrapper{
		builder: builder,
		metrics: metrics,
	}
}

func (w BlockBuilderMetricsWrapper) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error, sign func(*flow.Header) error) (*flow.Header, error) {
	processStart := time.Now()
	header, err := w.builder.BuildOn(parentID, setter, sign)
	w.metrics.PayloadProductionDuration(time.Since(processStart))
	return header, err
}
