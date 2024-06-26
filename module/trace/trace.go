package trace

import (
	"context"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/model/flow"
)

const DefaultEntityCacheSize = 1000

const SensitivityCaptureAll = 0
const EntityTypeBlock = "Block"
const EntityTypeCollection = "Collection"
const EntityTypeTransaction = "Transaction"

type SpanName string

func (s SpanName) Child(subOp string) SpanName {
	return SpanName(string(s) + "." + subOp)
}

func IsSampled(span trace.Span) bool {
	return span.SpanContext().IsSampled()
}

// Tracer is the implementation of the Tracer interface
// TODO(rbtz): make private
type Tracer struct {
	tracer      trace.Tracer
	shutdown    func(context.Context) error
	log         zerolog.Logger
	spanCache   *lru.Cache[flow.Identifier, trace.Span]
	chainID     string
	sensitivity uint
}

// NewTracer creates a new OpenTelemetry-based tracer.
func NewTracer(
	log zerolog.Logger,
	serviceName string,
	chainID string,
	sensitivity uint,
) (
	*Tracer,
	error,
) {
	ctx := context.TODO()
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// OLTP trace gRPC client initialization. Connection parameters for the exporter are extracted
	// from environment variables. e.g.: `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`.
	//
	// For more information, see OpenTelemetry specification:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.12.0/specification/protocol/exporter.md
	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		log.Debug().Err(err).Msg("tracing error")
	}))

	spanCache, err := lru.New[flow.Identifier, trace.Span](int(DefaultEntityCacheSize))
	if err != nil {
		return nil, err
	}

	return &Tracer{
		tracer:      tracerProvider.Tracer(""),
		shutdown:    tracerProvider.Shutdown,
		log:         log,
		spanCache:   spanCache,
		sensitivity: sensitivity,
		chainID:     chainID,
	}, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (t *Tracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *Tracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := t.shutdown(ctx); err != nil {
			t.log.Error().Err(err).Msg("failed to shutdown tracer")
		}

		t.spanCache.Purge()
		close(done)
	}()
	return done
}

func (t *Tracer) startEntitySpan(
	ctx context.Context,
	entityID flow.Identifier,
	entityType string,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	if !t.ShouldSample(entityID) {
		return NoopSpan, ctx
	}

	ctx, rootSpan := t.entityRootSpan(ctx, entityID, entityType)
	return t.StartSpanFromParent(rootSpan, spanName, opts...), ctx
}

// entityRootSpan returns the root span for the given entity from the cache
// and if not exist it would construct it and cache it and return it
// This should be used mostly for the very first span created for an entity on the service
func (t *Tracer) entityRootSpan(
	ctx context.Context,
	entityID flow.Identifier,
	entityType string,
) (
	context.Context,
	trace.Span,
) {
	if c, ok := t.spanCache.Get(entityID); ok {
		return trace.ContextWithSpan(ctx, c), c
	}

	traceID := (*trace.TraceID)(entityID[:16])
	spanConfig := trace.SpanContextConfig{
		TraceID:    *traceID,
		TraceFlags: trace.TraceFlags(0).WithSampled(true),
	}
	ctx = trace.ContextWithSpanContext(ctx, trace.NewSpanContext(spanConfig))
	ctx, span := t.tracer.Start(ctx, string(entityType))

	span.SetAttributes(
		attribute.String("entity_id", entityID.String()),
		attribute.String("chainID", t.chainID),
	)
	t.spanCache.Add(entityID, span)

	span.End() // end span right away
	return ctx, span
}

func (t *Tracer) BlockRootSpan(blockID flow.Identifier) trace.Span {
	_, span := t.entityRootSpan(context.Background(), blockID, EntityTypeBlock)
	return span
}

func (t *Tracer) StartBlockSpan(
	ctx context.Context,
	blockID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	return t.startEntitySpan(ctx, blockID, EntityTypeBlock, spanName, opts...)
}

func (t *Tracer) StartCollectionSpan(
	ctx context.Context,
	collectionID flow.Identifier,
	spanName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	return t.startEntitySpan(ctx, collectionID, EntityTypeCollection, spanName, opts...)
}

func (t *Tracer) StartSpanFromContext(
	ctx context.Context,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) (
	trace.Span,
	context.Context,
) {
	ctx, span := t.tracer.Start(ctx, string(operationName), opts...)
	return span, ctx
}

func (t *Tracer) StartSpanFromParent(
	parentSpan trace.Span,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) trace.Span {
	if !IsSampled(parentSpan) {
		return NoopSpan
	}

	ctx := trace.ContextWithSpan(context.Background(), parentSpan)
	_, span := t.tracer.Start(ctx, string(operationName), opts...)
	return span
}

func (t *Tracer) ShouldSample(entityID flow.Identifier) bool {
	return entityID.IsSampled(t.sensitivity)
}

func (t *Tracer) StartSampledSpanFromParent(
	parentSpan trace.Span,
	entityID flow.Identifier,
	operationName SpanName,
	opts ...trace.SpanStartOption,
) trace.Span {
	if !t.ShouldSample(entityID) {
		return NoopSpan
	}

	return t.StartSpanFromParent(parentSpan, operationName, opts...)
}

func (t *Tracer) RecordSpanFromParent(
	parentSpan trace.Span,
	operationName SpanName,
	duration time.Duration,
	attrs []attribute.KeyValue,
	opts ...trace.SpanStartOption,
) {
	if !IsSampled(parentSpan) {
		return
	}
	end := time.Now()
	start := end.Add(-duration)
	ctx := trace.ContextWithSpanContext(context.Background(), parentSpan.SpanContext())
	opts = append(opts,
		trace.WithAttributes(attrs...),
		trace.WithTimestamp(start),
	)
	_, span := t.tracer.Start(ctx, string(operationName), opts...)
	span.End(trace.WithTimestamp(end))
}

// WithSpanFromContext encapsulates executing a function within an span, i.e., it starts a span with the specified SpanName from the context,
// executes the function f, and finishes the span once the function returns.
func (t *Tracer) WithSpanFromContext(ctx context.Context,
	operationName SpanName,
	f func(),
	opts ...trace.SpanStartOption,
) {
	span, _ := t.StartSpanFromContext(ctx, operationName, opts...)
	defer span.End()

	f()
}
