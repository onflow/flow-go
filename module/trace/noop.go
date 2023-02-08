package trace

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
)

var (
	NoopSpan trace.Span = trace.SpanFromContext(context.Background())
)

// NoopTracer is the implementation of the Tracer interface.
// TODO(rbtz): make private
type NoopTracer struct{}

// NewTracer creates a new tracer.
func NewNoopTracer() *NoopTracer {
	return &NoopTracer{}
}

// Ready returns a channel that will close when the network stack is ready.
func (t *NoopTracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *NoopTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func (t *NoopTracer) BlockRootSpan(entityID flow.Identifier) trace.Span {
	return NoopSpan
}

func (t *NoopTracer) StartBlockSpan(
	ctx context.Context,
	entityID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	return NoopSpan, ctx
}

func (t *NoopTracer) StartCollectionSpan(
	ctx context.Context,
	entityID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	return NoopSpan, ctx
}

func (t *NoopTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	return NoopSpan, ctx
}

func (t *NoopTracer) StartSpanFromParent(
	parentSpan trace.Span,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) trace.Span {
	return NoopSpan
}

func (t *NoopTracer) ShouldSample(entityID flow.Identifier) bool {
	return true
}

func (t *NoopTracer) StartSampledSpanFromParent(
	parentSpan trace.Span,
	entityID flow.Identifier,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) trace.Span {
	return NoopSpan
}

func (t *NoopTracer) WithSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...trace.SpanStartOption,
) {
	f()
}
