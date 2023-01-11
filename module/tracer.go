package module

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

var (
	_ Tracer = &trace.Tracer{}
	_ Tracer = &trace.NoopTracer{}
	_ Tracer = &trace.LogTracer{}
)

// Tracer interface for tracers in flow. Uses open tracing span definitions
type Tracer interface {
	ReadyDoneAware

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

	// StartTransactionSpan starts an span for a transaction, built as a child
	// of rootSpan.  It also returns the context including this span which can
	// be used for nested calls.
	StartTransactionSpan(
		ctx context.Context,
		transactionID flow.Identifier,
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

	// RecordSpanFromParent records an span at finish time
	// start time will be computed by reducing time.Now() - duration
	RecordSpanFromParent(
		parentSpan otelTrace.Span,
		operationName trace.SpanName,
		duration time.Duration,
		attrs []attribute.KeyValue,
		opts ...otelTrace.SpanStartOption,
	)

	// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
	// executes the function f, and finishes the span once the function returns.
	WithSpanFromContext(
		ctx context.Context,
		operationName trace.SpanName,
		f func(),
		opts ...otelTrace.SpanStartOption,
	)
}
