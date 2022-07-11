package provider

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

// New registers a global tracer provider with a default OTLP exporter pointing to a collector.
// The global trace provider allows you initialize a new trace from anywhere in code.
// Use the trace provider to create a named tracer. E.g. tracer := otel.Tracer("example.com/foo")
func New(
	ctx context.Context,
	serviceName string,
	collectorEndpoint string,
	sampler trace.Sampler,
) (tp *trace.TracerProvider, err error) {
	r, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		// TODO use gcp.NewDetector() on next release
		resource.WithDetectors(&gcp.GKE{}),
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracing resource: %w", err)
	}

	traceOpts := []trace.TracerProviderOption{
		trace.WithSampler(sampler),
		trace.WithResource(r),
	}

	// Create default otlp exporter
	exporter, err := otlptrace.New(
		ctx,
		otlpgrpc.NewClient(
			otlpgrpc.WithInsecure(),
			otlpgrpc.WithEndpoint(collectorEndpoint),
			otlpgrpc.WithDialOption(grpc.WithBlock()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize trace exporter: %w", err)
	}

	traceOpts = append(traceOpts, trace.WithBatcher(exporter))

	// Create a new tracer provider with a batch span processor and the otlp exporters.
	tp = trace.NewTracerProvider(traceOpts...)
	// Set the Tracer Provider and the W3C Trace Context propagator as globals
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return
}

// Cleanup calls the TraceProvider to shutdown any span processors
func Cleanup(ctx context.Context, tp *trace.TracerProvider) {
	if tp != nil {
		tp.ForceFlush(ctx)
		_ = tp.Shutdown(ctx)
	}
}
