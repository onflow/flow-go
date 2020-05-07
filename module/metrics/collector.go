package metrics

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/module/trace"
)

// Collector is a metrics collector for monitoring purpose.
// It provides methods for collecting metrics data.
type Collector struct {
	tracer *trace.OpenTracer
}

func NewCollector(log zerolog.Logger) (*Collector, error) {
	tracer, err := trace.NewTracer(log, "tracer")
	if err != nil {
		return nil, err
	}

	return &Collector{
		tracer: tracer,
	}, nil
}
