package trace

import (
	"context"
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
)

type spanKey string

const activeSpan spanKey = "activeSpan"

// LogTracer is the implementation of the Tracer interface which passes
// all the traces back to the passed logger and print them
// this is mostly useful for debugging and testing
// TODO(rbtz): make private
type LogTracer struct {
	log      zerolog.Logger
	provider trace.TracerProvider
}

// NewLogTracer creates a new zerolog-based tracer.
// TODO: Consider switching to go.opentelemetry.io/otel/exporters/stdout/stdouttrace
func NewLogTracer(log zerolog.Logger) *LogTracer {
	t := &LogTracer{
		log: log,
	}
	t.provider = &logTracerProvider{tracer: t}
	return t
}

func (t *LogTracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

func (t *LogTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

// Start implements trace.Tracer interface.
func (t *LogTracer) Start(
	ctx context.Context,
	spanName string,
	_ ...trace.SpanStartOption,
) (
	context.Context,
	trace.Span,
) {
	sp := newLogSpan(t, spanName)
	ctx = context.WithValue(ctx, activeSpan, sp.spanID)
	return ctx, sp
}

func (t *LogTracer) StartBlockSpan(
	ctx context.Context,
	blockID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	ctx, sp := t.Start(ctx, string(spanName), opts...)
	return sp, ctx
}

func (t *LogTracer) StartCollectionSpan(
	ctx context.Context,
	collectionID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	ctx, sp := t.Start(ctx, string(spanName), opts...)
	return sp, ctx
}

// StartTransactionSpan starts a span that will be aggregated under the given transaction.
// All spans for the same transaction will be aggregated under a root span
func (t *LogTracer) StartTransactionSpan(
	ctx context.Context,
	transactionID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	ctx, sp := t.Start(ctx, string(spanName), opts...)
	return sp, ctx
}

func (t *LogTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	parentSpanID := ctx.Value(activeSpan).(uint64)
	sp := newLogSpanWithParent(t, operationName, parentSpanID)
	ctx = context.WithValue(ctx, activeSpan, sp.spanID)
	return sp, trace.ContextWithSpan(ctx, sp)
}

func (t *LogTracer) StartSpanFromParent(
	span trace.Span,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) trace.Span {
	parentSpan := span.(*logSpan)
	return newLogSpanWithParent(t, operationName, parentSpan.spanID)
}

func (t *LogTracer) RecordSpanFromParent(
	span trace.Span,
	operationName SpanName,
	duration time.Duration,
	attrs []attribute.KeyValue,
	opts ...trace.SpanStartOption,
) {
	parentSpan := span.(*logSpan)
	sp := newLogSpanWithParent(t, operationName, parentSpan.spanID)
	sp.start = time.Now().Add(-duration)
	span.End()
}

// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
// executes the function f, and finishes the span once the function returns.
func (t *LogTracer) WithSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...trace.SpanStartOption,
) {
	span, _ := t.StartSpanFromContext(ctx, operationName, opts...)
	defer span.End()

	f()
}

var _ trace.Span = &logSpan{}

type logSpan struct {
	tracer *LogTracer

	spanID        uint64
	parentID      uint64
	operationName string
	start         time.Time
	end           time.Time
	attrs         map[attribute.Key]attribute.Value
}

func newLogSpan(tracer *LogTracer, operationName string) *logSpan {
	return &logSpan{
		tracer:        tracer,
		spanID:        rand.Uint64(),
		operationName: operationName,
		start:         time.Now(),
		attrs:         make(map[attribute.Key]attribute.Value),
	}
}

func newLogSpanWithParent(
	tracer *LogTracer,
	operationName SpanName,
	parentSpanID uint64,
) *logSpan {
	sp := newLogSpan(tracer, string(operationName))
	sp.parentID = parentSpanID
	return sp
}

func (s *logSpan) produceLog() {
	s.tracer.log.Info().
		Uint64("spanID", s.spanID).
		Uint64("parent", s.parentID).
		Time("start", s.start).
		Time("end", s.end).
		TimeDiff("duration", s.end, s.start).
		Str("operationName", string(s.operationName)).
		Interface("attrs", s.attrs).
		Msg("span")
}

func (s *logSpan) End(...trace.SpanEndOption) {
	s.end = time.Now()
	s.produceLog()
}

func (s *logSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}

func (s *logSpan) IsRecording() bool {
	return s.end == time.Time{}
}

func (s *logSpan) SetStatus(codes.Code, string) {}

func (s *logSpan) SetError(bool) {}

func (s *logSpan) SetAttributes(attrs ...attribute.KeyValue) {
	for _, f := range attrs {
		s.attrs[f.Key] = f.Value
	}
}

func (s *logSpan) RecordError(error, ...trace.EventOption) {}

func (s *logSpan) AddEvent(string, ...trace.EventOption) {}

func (s *logSpan) SetName(string) {}

func (s *logSpan) TracerProvider() trace.TracerProvider {
	return s.tracer.provider
}

type logTracerProvider struct {
	tracer *LogTracer
}

func (ltp *logTracerProvider) Tracer(
	instrumentationName string,
	_ ...trace.TracerOption,
) trace.Tracer {
	return ltp.tracer
}
