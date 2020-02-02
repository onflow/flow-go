package trace

import (
	opentracing "github.com/opentracing/opentracing-go"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Tracer interface for tracers in flow. Uses open tracing span definitions
type Tracer interface {
	module.ReadyDoneAware
	StartSpan(entity flow.Identifier, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span
	FinishSpan(entity flow.Identifier)
	GetSpan(entity flow.Identifier) (opentracing.Span, bool)
}
