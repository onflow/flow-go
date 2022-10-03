package environment

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

type TracerParams struct {
	module.Tracer
	ExtensiveTracing bool

	RootSpan otelTrace.Span
}

func DefaultTracerParams() TracerParams {
	// NOTE: RootSpan is set by NewTransactionEnv rather by Context
	return TracerParams{
		Tracer:           nil,
		ExtensiveTracing: false,
	}
}

type Tracer struct {
	TracerParams
}

func NewTracer(params TracerParams) *Tracer {
	return &Tracer{params}
}

func (tracer *Tracer) isTraceable() bool {
	return tracer.Tracer != nil && tracer.RootSpan != nil
}

func (tracer *Tracer) StartSpanFromRoot(name trace.SpanName) otelTrace.Span {
	if tracer.isTraceable() {
		return tracer.Tracer.StartSpanFromParent(tracer.RootSpan, name)
	}

	return trace.NoopSpan
}

func (tracer *Tracer) StartExtensiveTracingSpanFromRoot(
	name trace.SpanName,
) otelTrace.Span {
	if tracer.isTraceable() && tracer.ExtensiveTracing {
		return tracer.Tracer.StartSpanFromParent(tracer.RootSpan, name)
	}

	return trace.NoopSpan
}

func (tracer *Tracer) RecordSpanFromRoot(
	spanName trace.SpanName,
	duration time.Duration,
	attrs []attribute.KeyValue,
) {
	if !tracer.isTraceable() {
		return
	}
	tracer.Tracer.RecordSpanFromParent(
		tracer.RootSpan,
		spanName,
		duration,
		attrs)
}
