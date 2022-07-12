package trace

import (
	"context"
	"io"
	"math/rand"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"
	"go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
)

// OtelTracer is the implementation of the Tracer interface
type OtelTracer struct {
	trace.Tracer
	closer      io.Closer
	log         zerolog.Logger
	spanCache   *lru.Cache
	sensitivity uint
	chainID     string
}

// NewOtelTracer creates a new tracer.
//
// TODO (ramtin) : we might need to add a mutex lock (not sure if tracer itself is thread-safe)
func NewOtelTracer(log zerolog.Logger,
	serviceName string,
	chainID string,
	sensitivity uint) (*OtelTracer, error) {

	return nil, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (t *OtelTracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *OtelTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		t.closer.Close()
		close(done)
	}()
	return done
}

// entityRootSpan returns the root span for the given entity from the cache
// and if not exist it would construct it and cache it and return it
// This should be used mostly for the very first span created for an entity on the service
func (t *OtelTracer) entityRootSpan(entityID flow.Identifier, entityType string, opts ...trace.SpanStartOption) trace.Span {
	if span, ok := t.spanCache.Get(entityID); ok {
		return span.(trace.Span)
	}
	// flow.Identifier to flow
	traceID, err := trace.TraceIDFromHex(entityID.String()[:32]) //get traceID from hex
	if err != nil {
		sp, _ := t.StartSpanFromContext(context.Background(), "entity tracing started")
		return sp
	}

	spanID, err := trace.SpanIDFromHex(entityID.String()[:32]) //get traceID from hex
	if err != nil {
		sp, _ := t.StartSpanFromContext(context.Background(), "entity tracing started")
		return sp
	}

	traceState, err := trace.ParseTraceState(entityID.String()[:32])
	if err != nil {
		sp, _ := t.StartSpanFromContext(context.Background(), "entity tracing started")
		return sp
	}
 

	ctx := trace.ContextWithRemoteSpanContext(ctx, trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceState: traceState,
		TraceFlags: 1,
	}))
	// opts = append(opts, jaeger.SelfRef(ctx))
	span := t.Tracer.StartSpan(string(entityType), opts...)
	// keep full entityID
	span.LogFields(log.String("entity_id", entityID.String()))
	// set chainID as tag for filtering traces from different networks
	span.SetTag("chainID", t.chainID)
	t.spanCache.Add(entityID, span)

	span.Finish() // finish span right away
	return span
}

func (t *OtelTracer) StartBlockSpan(
	ctx context.Context,
	blockID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption) (trace.Span, context.Context, bool) {

	if !blockID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(blockID, EntityTypeBlock)
	ctx = trace.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

func (t *OtelTracer) StartCollectionSpan(
	ctx context.Context,
	collectionID flow.Identifier,
	spanName SpanName,
	opts ...trace.StartSpanOption) (trace.Span, context.Context, bool) {

	if !collectionID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(collectionID, EntityTypeCollection)
	ctx = trace.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

// StartTransactionSpan starts a span that will be aggregated under the given transaction.
// All spans for the same transaction will be aggregated under a root span
func (t *OtelTracer) StartTransactionSpan(
	ctx context.Context,
	transactionID flow.Identifier,
	spanName SpanName,
	opts ...trace.StartSpanOption) (trace.Span, context.Context, bool) {

	if !transactionID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(transactionID, EntityTypeTransaction)
	ctx = trace.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

func (t *OtelTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...trace.StartSpanOption,
) (trace.Span, context.Context) {
	parentSpan := trace.SpanFromContext(ctx)
	if parentSpan == nil {
		return &NoopSpan{&NoopTracer{}}, ctx
	}
	if _, ok := parentSpan.(*NoopSpan); ok {
		return &NoopSpan{&NoopTracer{}}, ctx
	}

	opts = append(opts, trace.ChildOf(parentSpan.Context()))
	span := t.Tracer.StartSpan(string(operationName), opts...)
	return span, trace.ContextWithSpan(ctx, span)
}

func (t *OtelTracer) StartSpanFromParent(
	span trace.Span,
	operationName SpanName,
	opts ...trace.StartSpanOption,
) trace.Span {
	if _, ok := span.(*NoopSpan); ok {
		return &NoopSpan{&NoopTracer{}}
	}
	opts = append(opts, trace.ChildOf(span.Context()))
	return t.Tracer.StartSpan(string(operationName), opts...)
}

func (t *OtelTracer) RecordSpanFromParent(
	span trace.Span,
	operationName SpanName,
	duration time.Duration,
	logs []trace.LogRecord,
	opts ...trace.StartSpanOption,
) {
	if _, ok := span.(*NoopSpan); ok {
		return
	}
	end := time.Now()
	start := end.Add(-duration)
	opts = append(opts, trace.FollowsFrom(span.Context()))
	opts = append(opts, trace.StartTime(start))
	sp := t.Tracer.StartSpan(string(operationName), opts...)
	sp.FinishWithOptions(trace.FinishOptions{FinishTime: end, LogRecords: logs})
}

// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
// executes the function f, and finishes the span once the function returns.
func (t *OtelTracer) WithSpanFromContext(ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...trace.StartSpanOption) {
	span, _ := t.StartSpanFromContext(ctx, operationName, opts...)
	defer span.Finish()

	f()
}
