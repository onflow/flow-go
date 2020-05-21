package blockproducer

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// BlockBuilderMetricsWrapper measures the time which the HotStuff's core logic
// spends in the module.Builder component, i.e. the with generating block payloads
type BlockBuilderMetricsWrapper struct {
	builder module.Builder
	metrics module.HotstuffMetrics
}

func NewMetricsWrapper(builder module.Builder, metrics module.HotstuffMetrics) module.Builder {
	return &BlockBuilderMetricsWrapper{
		builder: builder,
		metrics: metrics,
	}
}

func (w BlockBuilderMetricsWrapper) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	processStart := time.Now()
	header, err := w.builder.BuildOn(parentID, setter)
	w.metrics.PayloadProductionDuration(time.Since(processStart))
	return header, err
}
