package fvm

import (
	"github.com/opentracing/opentracing-go"

	"github.com/onflow/flow-go/module/trace"
)

type Span interface {
	// TODO(patrick/rbtz): switch to opentelemtry
	opentracing.Span

	// TODO(patrick/rbtz): remove once switched to opentelemtry (#2823)
	End()

	IsNoOp() bool
}

type noOpSpan struct {
	trace.NoopSpan
}

func (noOpSpan) IsNoOp() bool { return true }

func (noOpSpan) End() {}

type realSpan struct {
	opentracing.Span
}

func (realSpan) IsNoOp() bool { return false }

// TODO(patrick/rbtz): remove once switched to opentelemtry (#2823)
func (span *realSpan) End() {
	span.Finish()
}

// NOTE: This dummy struct is used to avoid naming collision between
// Context() and anonymous field in EnvContext
type nestedContext struct {
	Context
}

type EnvContext struct {
	nestedContext

	rootSpan opentracing.Span
}

func (ctx *EnvContext) Context() *Context {
	return &ctx.nestedContext.Context
}

func (ctx *EnvContext) isTraceable() bool {
	return ctx.Tracer != nil && ctx.rootSpan != nil
}

func (ctx *EnvContext) StartSpanFromRoot(name trace.SpanName) Span {
	if ctx.isTraceable() {
		return &realSpan{
			ctx.Tracer.StartSpanFromParent(ctx.rootSpan, name),
		}
	}

	return &noOpSpan{}
}

func (ctx *EnvContext) StartExtensiveTracingSpanFromRoot(name trace.SpanName) Span {
	if ctx.isTraceable() && ctx.ExtensiveTracing {
		return &realSpan{
			ctx.Tracer.StartSpanFromParent(ctx.rootSpan, name),
		}
	}

	return &noOpSpan{}
}
