package tracing

import opentracing "github.com/opentracing/opentracing-go"

type Traceable struct {
	span opentracing.Span
}

func (t *Traceable) StartSpan(tracer opentracing.Tracer, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	t.span = tracer.StartSpan(spanName, opts...)
	return t.span
}
func (t *Traceable) FinishSpan() {
	if t.span == nil {
		return
	}
	t.span.Finish()
}

func (t *Traceable) SetTag(key string, value interface{}) {
	if t.span == nil {
		return
	}
	t.span.SetTag(key, value)
}

func (t *Traceable) SpanContext() opentracing.SpanContext {
	return t.span.Context()
}
