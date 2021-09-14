package trace

import (
	"context"
	"io"
	"math/rand"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"

	"github.com/onflow/flow-go/model/flow"
)

const DefaultEntityCacheSize = 1000

const SensitivityCaptureAll = 0
const EntityTypeBlock = "Block"
const EntityTypeCollection = "Collection"
const EntityTypeTransaction = "Transaction"

type SpanName string

// OpenTracer is the implementation of the Tracer interface
type OpenTracer struct {
	opentracing.Tracer
	closer      io.Closer
	log         zerolog.Logger
	spanCache   *lru.Cache
	sensitivity uint
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
//
// TODO (ramtin) : we might need to add a mutex lock (not sure if tracer itself is thread-safe)
func NewTracer(log zerolog.Logger, serviceName string, sensitivity uint) (*OpenTracer, error) {
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

	spanCache, err := lru.New(int(DefaultEntityCacheSize))
	if err != nil {
		return nil, err
	}

	t := &OpenTracer{
		Tracer:      tracer,
		closer:      closer,
		log:         log,
		spanCache:   spanCache,
		sensitivity: sensitivity,
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

// entityRootSpan returns the root span for the given entity from the cache
// and if not exist it would construct it and cache it and return it
// This should be used mostly for the very first span created for an entity on the service
func (t *OpenTracer) entityRootSpan(entityID flow.Identifier, entityType string, opts ...opentracing.StartSpanOption) opentracing.Span {
	if span, ok := t.spanCache.Get(entityID); ok {
		return span.(opentracing.Span)
	}

	// flow.Identifier to flow
	traceID, err := jaeger.TraceIDFromString(entityID.String()[:32])
	if err != nil {
		// don't panic, gracefully move forward with background context
		sp, _ := t.StartSpanFromContext(context.Background(), "entity tracing started")
		return sp
	}

	ctx := jaeger.NewSpanContext(
		traceID,
		jaeger.SpanID(rand.Uint64()),
		jaeger.SpanID(0),
		true,
		nil,
	)
	opts = append(opts, jaeger.SelfRef(ctx))
	span := t.Tracer.StartSpan(string(entityType), opts...)
	t.spanCache.Add(entityID, span)
	span.Finish() // finish span right away
	return span
}

func (t *OpenTracer) StartBlockSpan(
	ctx context.Context,
	blockID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {

	if !blockID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(blockID, EntityTypeBlock)
	ctx = opentracing.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

func (t *OpenTracer) StartCollectionSpan(
	ctx context.Context,
	collectionID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {

	if !collectionID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(collectionID, EntityTypeCollection)
	ctx = opentracing.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

func (t *OpenTracer) StartTransactionSpan(
	ctx context.Context,
	transactionID flow.Identifier,
	spanName SpanName,
	opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context, bool) {

	if !transactionID.IsSampled(t.sensitivity) {
		return &NoopSpan{&NoopTracer{}}, ctx, false
	}

	rootSpan := t.entityRootSpan(transactionID, EntityTypeTransaction)
	ctx = opentracing.ContextWithSpan(ctx, rootSpan)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx, true
}

func (t *OpenTracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) (opentracing.Span, context.Context) {
	parentSpan := opentracing.SpanFromContext(ctx)
	if parentSpan == nil {
		return &NoopSpan{&NoopTracer{}}, ctx
	}
	if _, ok := parentSpan.(*NoopSpan); ok {
		return &NoopSpan{&NoopTracer{}}, ctx
	}

	opts = append(opts, opentracing.ChildOf(parentSpan.Context()))
	span := t.Tracer.StartSpan(string(operationName), opts...)
	return span, opentracing.ContextWithSpan(ctx, span)
}

func (t *OpenTracer) StartSpanFromParent(
	span opentracing.Span,
	operationName SpanName,
	opts ...opentracing.StartSpanOption,
) opentracing.Span {
	if _, ok := span.(*NoopSpan); ok {
		return &NoopSpan{&NoopTracer{}}
	}
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
	if _, ok := span.(*NoopSpan); ok {
		return
	}
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
