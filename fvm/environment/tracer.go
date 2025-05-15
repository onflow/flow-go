package environment

import (
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
)

// Tracer captures traces
type Tracer interface {
	StartChildSpan(
		name trace.SpanName,
		options ...otelTrace.SpanStartOption,
	) tracing.TracerSpan
}
