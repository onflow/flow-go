package fvm

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/module/trace"
)

// NOTE: This dummy struct is used to avoid naming collision between
// Context() and anonymous field in EnvContext
type nestedContext struct {
	Context
}

type EnvContext struct {
	nestedContext

	rootSpan otelTrace.Span
}

func NewEnvContext(ctx Context, root otelTrace.Span) *EnvContext {
	return &EnvContext{nestedContext{ctx}, root}
}

func (ctx *EnvContext) Context() *Context {
	return &ctx.nestedContext.Context
}

func (ctx *EnvContext) isTraceable() bool {
	return ctx.Tracer != nil && ctx.rootSpan != nil
}

func (ctx *EnvContext) StartSpanFromRoot(name trace.SpanName) otelTrace.Span {
	if ctx.isTraceable() {
		return ctx.Tracer.StartSpanFromParent(ctx.rootSpan, name)
	}

	return trace.NoopSpan
}

func (ctx *EnvContext) StartExtensiveTracingSpanFromRoot(name trace.SpanName) otelTrace.Span {
	if ctx.isTraceable() && ctx.ExtensiveTracing {
		return ctx.Tracer.StartSpanFromParent(ctx.rootSpan, name)
	}

	return trace.NoopSpan
}

func (ctx *EnvContext) RecordSpanFromRoot(
	spanName trace.SpanName,
	duration time.Duration,
	attrs []attribute.KeyValue,
) {
	if !ctx.isTraceable() {
		return
	}
	ctx.Tracer.RecordSpanFromParent(ctx.rootSpan, spanName, duration, attrs)
}
