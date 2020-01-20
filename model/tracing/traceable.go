package tracing

import (
	opentracing "github.com/opentracing/opentracing-go"
)

type Traceable struct {
	span opentracing.Span
}

func (t *Traceable) StartSpan(tracer opentracing.Tracer, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	if t == nil {
		t = new(Traceable)
	}
	t.span = tracer.StartSpan(spanName, opts...)
	return t.span
}
func (t *Traceable) FinishSpan() {
	if t == nil || t.span == nil {
		return
	}
	t.span.Finish()
}

func (t *Traceable) SetTag(key string, value interface{}) {
	if t == nil || t.span == nil {
		return
	}
	t.span.SetTag(key, value)
}

func (t *Traceable) SpanContext() opentracing.SpanContext {
	if t == nil || t.span == nil {
		return nil
	}
	return t.span.Context()
}
