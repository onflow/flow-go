package propagation

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var traceHeaderKeys = []string{"traceparent", "tracestate", "baggage"}

// TraceHeaderKeys returns a list of trace headers
func TraceHeaderKeys() []string {
	return traceHeaderKeys
}

// ContextAndSpanFromHeaders returns children context and span restored from serialized context
// in the form of W3C trace context headers https://www.w3.org/TR/trace-context/ and https://www.w3.org/TR/baggage
// headers is a map[string][]byte that may contain traceparent, tracestate or baggage keys.
func ContextAndSpanFromHeaders(ctx context.Context, spanName string, headers map[string][]byte, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	b := new(propagation.Baggage)

	traceHeaders := make(map[string]string)
	for _, k := range traceHeaderKeys {
		traceHeaders[k] = string(headers[k])
	}
	mc := propagation.MapCarrier(traceHeaders)

	// Add traceparent and tracestate to context
	parentCtx := otel.GetTextMapPropagator().Extract(ctx, mc)
	// Create remote span context
	remoteSpanCtx := trace.SpanContextFromContext(parentCtx)
	// Create child span
	tracedCtx, span := otel.
		Tracer("").
		Start(
			trace.ContextWithRemoteSpanContext(ctx, remoteSpanCtx),
			spanName,
			opts...,
		)

	// Add baggage to context
	tracedCtxWithBaggage := b.Extract(tracedCtx, mc)

	return tracedCtxWithBaggage, span
}

// HeadersFromContext injects tracing data from a context into a carrier
// to facilitate trace context serialization
func HeadersFromContext(ctx context.Context) map[string][]byte {
	mc := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, mc)
	new(propagation.Baggage).Inject(ctx, mc)

	o := make(map[string][]byte)
	for _, k := range traceHeaderKeys {
		if s, ok := mc[k]; ok {
			o[k] = []byte(s)
		}
	}

	return o
}
