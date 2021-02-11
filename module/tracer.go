package module

import (
	"context"

	"github.com/opentracing/opentracing-go"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var _ Tracer = &trace.OpenTracer{}

// Tracer interface for tracers in flow. Uses open tracing span definitions
type Tracer interface {
	ReadyDoneAware
	StartSpan(entity flow.Identifier, spanName trace.SpanName, opts ...opentracing.StartSpanOption) opentracing.Span
	FinishSpan(entity flow.Identifier, spanName trace.SpanName)
	GetSpan(entity flow.Identifier, spanName trace.SpanName) (opentracing.Span, bool)

	StartSpanFromContext(
		ctx context.Context,
		operationName trace.SpanName,
		opts ...opentracing.StartSpanOption,
	) (opentracing.Span, context.Context)

	StartSpanFromParent(
		span opentracing.Span,
		operationName trace.SpanName,
		opts ...opentracing.StartSpanOption,
	) opentracing.Span

	// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
	// executes the function f, and finishes the span once the function returns.
	WithSpanFromContext(ctx context.Context,
		operationName trace.SpanName,
		f func(),
		opts ...opentracing.StartSpanOption)
}
