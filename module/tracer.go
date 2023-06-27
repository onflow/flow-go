package module

import (
	"context"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var (
	_ Tracer = &trace.Tracer{}
	_ Tracer = &trace.NoopTracer{}
)

// Tracer interface for tracers in flow. Uses open tracing span definitions
type Tracer interface {
	ReadyDoneAware

	// BlockRootSpan returns the block's empty root span.  The returned span
	// has already ended, and should only be used for creating children span.
	BlockRootSpan(blockID flow.Identifier) otelTrace.Span

	// StartBlockSpan starts an span for a block, built as a child of rootSpan.
	// It also returns the context including this span which can be used for
	// nested calls.
	StartBlockSpan(
		ctx context.Context,
		blockID flow.Identifier,
		spanName trace.SpanName,
		opts ...otelTrace.SpanStartOption,
	) (
		otelTrace.Span,
		context.Context,
	)

	// StartCollectionSpan starts an span for a collection, built as a child of
	// rootSpan.  It also returns the context including this span which can be
	// used for nested calls.
	StartCollectionSpan(
		ctx context.Context,
		collectionID flow.Identifier,
		spanName trace.SpanName,
		opts ...otelTrace.SpanStartOption,
	) (
		otelTrace.Span,
		context.Context,
	)

	StartSpanFromContext(
		ctx context.Context,
		operationName trace.SpanName,
		opts ...otelTrace.SpanStartOption,
	) (
		otelTrace.Span,
		context.Context,
	)

	StartSpanFromParent(
		parentSpan otelTrace.Span,
		operationName trace.SpanName,
		opts ...otelTrace.SpanStartOption,
	) otelTrace.Span

	ShouldSample(entityID flow.Identifier) bool

	// StartSampledSpanFromParent starts a real span from the parent span
	// if the entity should be sampled.  Otherwise, it returns a no-op span.
	StartSampledSpanFromParent(
		parentSpan otelTrace.Span,
		entityID flow.Identifier,
		operationName trace.SpanName,
		opts ...otelTrace.SpanStartOption,
	) otelTrace.Span

	// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
	// executes the function f, and finishes the span once the function returns.
	WithSpanFromContext(
		ctx context.Context,
		operationName trace.SpanName,
		f func(),
		opts ...otelTrace.SpanStartOption,
	)
}
