package trace

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"

	"github.com/onflow/flow-go/model/flow"
)

type SpanName string

func (s SpanName) Child(subOp string) SpanName {
	return SpanName(string(s) + "." + subOp)
}

// OpenTracer is the implementation of the Tracer interface
type OpenTracer struct {
	opentracing.Tracer
	closer    io.Closer
	log       zerolog.Logger
	openSpans map[string]opentracing.Span
	lock      sync.RWMutex
}

type traceLogger struct {
	zerolog.Logger
}

func (t traceLogger) Error(msg string) {
	t.Logger.Error().Msg(msg)
}

// Infof logs a message at info priority
func (t traceLogger) Infof(msg string, args ...interface{}) {
	t.Debug().Msgf(msg, args...)
}

// NewTracer creates a new tracer.
func NewTracer(log zerolog.Logger, serviceName string) (*OpenTracer, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}

	if cfg.ServiceName == "" {
		cfg.ServiceName = serviceName
	}

	tracer, closer, err := cfg.NewTracer(config.Logger(traceLogger{log}))
	if err != nil {
		return nil, err
	}

	t := &OpenTracer{
		Tracer:    tracer,
		closer:    closer,
		log:       log,
		openSpans: map[string]opentracing.Span{},
	}

	return t, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (t *OpenTracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *OpenTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		t.closer.Close()
		close(done)
	}()
	return done
}

// StartSpan starts a span using the flow identifier as a key into the span map
func (t *OpenTracer) StartSpan(entityID flow.Identifier, spanName SpanName, opts ...opentracing.StartSpanOption) opentracing.Span {
	t.lock.Lock()
	defer t.lock.Unlock()
	key := spanKey(entityID, spanName)
	t.openSpans[key] = t.Tracer.StartSpan(string(spanName), opts...)
	return t.openSpans[key]
}

// FinishSpan finishes a span started with the passed in flow identifier
func (t *OpenTracer) FinishSpan(entityID flow.Identifier, spanName SpanName) {
	t.lock.Lock()
	defer t.lock.Unlock()
	key := spanKey(entityID, spanName)
	span, ok := t.openSpans[key]
	if ok {
		span.Finish()
		jaegerSpan := span.(*jaeger.Span)
		spanDurationMetric.WithLabelValues(jaegerSpan.OperationName()).Observe(jaegerSpan.Duration().Seconds())
		delete(t.openSpans, key)
	}
}

// GetSpan will get the span started with the passed in flow identifier
func (t *OpenTracer) GetSpan(entityID flow.Identifier, spanName SpanName) (opentracing.Span, bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()
	key := spanKey(entityID, spanName)
	span, exists := t.openSpans[key]
	return span, exists
}

func (t *OpenTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) (opentracing.Span, context.Context) {
	return opentracing.StartSpanFromContextWithTracer(ctx, t.Tracer, string(operationName), opts...)
}

func (t *OpenTracer) StartSpanFromParent(
	span opentracing.Span,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) opentracing.Span {
	opts = append(opts, opentracing.FollowsFrom(span.Context()))
	return t.Tracer.StartSpan(string(operationName), opts...)
}

func (t *OpenTracer) RecordSpanFromParent(
	span opentracing.Span,
	operationName SpanName,
	duration time.Duration,
	logs []opentracing.LogRecord,
	opts ...opentracing.StartSpanOption,
) {
	t.lock.Lock()
	defer t.lock.Unlock()
	end := time.Now()
	start := end.Add(-duration)
	opts = append(opts, opentracing.FollowsFrom(span.Context()))
	opts = append(opts, opentracing.StartTime(start))
	sp := t.Tracer.StartSpan(string(operationName), opts...)
	sp.FinishWithOptions(opentracing.FinishOptions{FinishTime: end, LogRecords: logs})
}

// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
// executes the function f, and finishes the span once the function returns.
func (t *OpenTracer) WithSpanFromContext(ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...opentracing.StartSpanOption) {
	span, _ := t.StartSpanFromContext(ctx, operationName, opts...)
	defer span.Finish()

	f()
}

// in order to avoid different spans using the same entityID as the key, which creates a conflict,
// we use span name and entity id as the key for a span.
func spanKey(entityID flow.Identifier, spanName SpanName) string {
	return fmt.Sprintf("%s-%x", spanName, entityID)
}
