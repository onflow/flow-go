package propagation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/dapperlabs/tracing/otel/testutils"
	"go.opentelemetry.io/otel/propagation"
	otrace "go.opentelemetry.io/otel/trace"
)

func TestContextAndSpanFromHeaders(t *testing.T) {
	ctx := context.Background()
	buff := &bytes.Buffer{}
	tp := testutils.RegisterTracerProvider(ctx, buff)
	defer tp.Shutdown(ctx)

	var (
		traceID      = "4bf92f3577b34da6a3ce929d0e0e4736"
		parentID     = "00f067aa0ba902b7"
		traceFlags   = "01"
		traceState   = "key1=value1,key2=value2"
		baggageValue = "key1=value1"
	)

	headers := map[string][]byte{
		"traceparent": []byte(fmt.Sprintf("00-%s-%s-%s", traceID, parentID, traceFlags)),
		"tracestate":  []byte(traceState),
		"baggage":     []byte(baggageValue),
	}
	ctx, span := ContextAndSpanFromHeaders(ctx, "test-span", headers)
	mc := propagation.MapCarrier(make(map[string]string))
	new(propagation.Baggage).Inject(ctx, mc)

	span.End()
	tp.ForceFlush(ctx)

	var output struct {
		Parent map[string]string
	}

	json.Unmarshal(buff.Bytes(), &output)

	if output.Parent["TraceID"] != traceID {
		t.Errorf("failed to set parent context traceID, wanted %s got %s", traceID, output.Parent["TraceID"])
	}
	if output.Parent["SpanID"] != parentID {
		t.Errorf("failed to set parent context spanID, wanted %s got %s", parentID, output.Parent["SpanID"])
	}
	if output.Parent["TraceState"] != traceState {
		t.Errorf("failed to set parent context tracestate, wanted %s got %s", traceState, output.Parent["TraceState"])
	}

	if mc["baggage"] != baggageValue {
		t.Errorf("failed to set context baggage, wanted %s got %s", baggageValue, mc["baggage"])
	}
}

func TestHeadersFromContext(t *testing.T) {
	ctx := context.Background()
	buff := &bytes.Buffer{}
	tp := testutils.RegisterTracerProvider(ctx, buff)
	defer tp.Shutdown(ctx)

	var (
		traceID, _ = otrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
		spanID, _  = otrace.SpanIDFromHex("00f067aa0ba902b7")
		traceFlags = "01"
		traceState = "key=value"
	)

	ts, _ := otrace.ParseTraceState(traceState)
	ctx = otrace.ContextWithRemoteSpanContext(ctx, otrace.NewSpanContext(otrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceState: ts,
		TraceFlags: 1,
	}))

	traceparent := fmt.Sprintf("00-%s-%s-%s", traceID, spanID, traceFlags)

	headers := HeadersFromContext(ctx)
	if string(headers["traceparent"]) != traceparent {
		t.Errorf("failed to extract traceparent from context, wanted %s got %s", traceparent, headers["traceparent"])
	}
	if string(headers["tracestate"]) != traceState {
		t.Errorf("failed to extract tracestate from context, wanted %s got %s", traceState, headers["tracestate"])
	}
}
