package trace

import (
	"context"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"

	"github.com/dapperlabs/flow-go/model/flow"
)

// NoopTracer is the implementation of the Tracer interface
type NoopTracer struct {
	closer io.Closer
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
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *NoopTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		t.closer.Close()
		close(done)
	}()
	return done
}

// StartSpan starts a span using the flow identifier as a key into the span map
func (t *NoopTracer) StartSpan(entityID flow.Identifier, spanName SpanName, opts ...opentracing.StartSpanOption,
) opentracing.Span {
	return &NoopSpan{t}
}

// FinishSpan finishes a span started with the passed in flow identifier
func (t *NoopTracer) FinishSpan(entityID flow.Identifier, spanName SpanName) {}

// GetSpan will get the span started with the passed in flow identifier
func (t *NoopTracer) GetSpan(entityID flow.Identifier, spanName SpanName) (opentracing.Span, bool) {
	return nil, false
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
