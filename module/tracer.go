package module

import (
	"context"

	"github.com/opentracing/opentracing-go"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Tracer interface for tracers in flow. Uses open tracing span definitions
type Tracer interface {
	ReadyDoneAware
	StartSpan(entity flow.Identifier, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span
	FinishSpan(entity flow.Identifier, spanName string)
	GetSpan(entity flow.Identifier, spanName string) (opentracing.Span, bool)

	StartSpanFromContext(
		ctx context.Context,
		operationName string,
		opts ...opentracing.StartSpanOption,
	) (opentracing.Span, context.Context)

	StartSpanFromParent(
		span opentracing.Span,
		operationName string,
		opts ...opentracing.StartSpanOption,
	) opentracing.Span
}
