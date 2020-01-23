package trace

import (
	"github.com/dapperlabs/flow-go/model/flow"
	opentracing "github.com/opentracing/opentracing-go"
)

type Tracer interface {
	StartSpan(entity flow.Identifier, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span
	FinishSpan(entity flow.Identifier)
	GetSpan(entity flow.Identifier) (opentracing.Span, bool)
	Ready() <-chan struct{}
	Done() <-chan struct{}
}
