package fvm

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
)

type Tracer struct {
	module.Tracer

	rootSpan         otelTrace.Span
	extensiveTracing bool
}

func NewTracer(
	tracer module.Tracer,
	root otelTrace.Span,
	extensiveTracing bool) *Tracer {
	return &Tracer{tracer, root, extensiveTracing}
}

func (tracer *Tracer) isTraceable() bool {
	return tracer.Tracer != nil && tracer.rootSpan != nil
}

func (tracer *Tracer) StartSpanFromRoot(name trace.SpanName) otelTrace.Span {
	if tracer.isTraceable() {
		return tracer.Tracer.StartSpanFromParent(tracer.rootSpan, name)
	}

	return trace.NoopSpan
}

func (tracer *Tracer) StartExtensiveTracingSpanFromRoot(name trace.SpanName) otelTrace.Span {
	if tracer.isTraceable() && tracer.extensiveTracing {
		return tracer.Tracer.StartSpanFromParent(tracer.rootSpan, name)
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
	tracer.Tracer.RecordSpanFromParent(tracer.rootSpan, spanName, duration, attrs)
}
