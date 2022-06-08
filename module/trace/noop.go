package trace

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"

	"github.com/onflow/flow-go/model/flow"
)

// NoopTracer is the implementation of the Tracer interface
type NoopTracer struct {
	tracer *InternalTracer
}

// NewTracer creates a new tracer.
func NewNoopTracer() *NoopTracer {
	t := &NoopTracer{}
	t.tracer = &InternalTracer{t}
	return t
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

func (t *NoopTracer) StartBlockSpan(
	ctx context.Context,
	entityID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {
	return &NoopSpan{t}, ctx, false
}

func (t *NoopTracer) StartCollectionSpan(
	ctx context.Context,
	entityID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {
	return &NoopSpan{t}, ctx, false
}

func (t *NoopTracer) StartTransactionSpan(
	ctx context.Context,
	entityID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {
	return &NoopSpan{t}, ctx, false
}

func (t *NoopTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) (opentracing.Span, context.Context) {
	return &NoopSpan{t}, ctx
}

func (t *NoopTracer) StartSpanFromParent(
	span opentracing.Span,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) opentracing.Span {
	return &NoopSpan{t}
}

func (t *NoopTracer) RecordSpanFromParent(
	span opentracing.Span,
	operationName SpanName,
	duration time.Duration,
	logs []opentracing.LogRecord,
	opts ...opentracing.StartSpanOption,
) {
}

func (t *NoopTracer) WithSpanFromContext(ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...opentracing.StartSpanOption) {
	f()
}

type InternalTracer struct {
	tracer *NoopTracer
}

func (t *InternalTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return &NoopSpan{t.tracer}
}

func (t *InternalTracer) Inject(sm opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return nil
}

func (t *InternalTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return &NoopSpanContext{}, nil
}

type NoopSpan struct {
	tracer *NoopTracer
}

func (s *NoopSpan) Finish()                                                     {}
func (s *NoopSpan) FinishWithOptions(opts opentracing.FinishOptions)            {}
func (s *NoopSpan) Context() opentracing.SpanContext                            { return &NoopSpanContext{} }
func (s *NoopSpan) SetOperationName(operationName string) opentracing.Span      { return s }
func (s *NoopSpan) SetTag(key string, value interface{}) opentracing.Span       { return s }
func (s *NoopSpan) LogFields(fields ...log.Field)                               {}
func (s *NoopSpan) LogKV(alternatingKeyValues ...interface{})                   {}
func (s *NoopSpan) SetBaggageItem(restrictedKey, value string) opentracing.Span { return s }
func (s *NoopSpan) BaggageItem(restrictedKey string) string                     { return "" }
func (s *NoopSpan) Tracer() opentracing.Tracer                                  { return s.tracer.tracer }
func (s *NoopSpan) LogEvent(event string)                                       {}
func (s *NoopSpan) LogEventWithPayload(event string, payload interface{})       {}
func (s *NoopSpan) Log(data opentracing.LogData)                                {}

type NoopSpanContext struct{}

func (c *NoopSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}
