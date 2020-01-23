package trace

import (
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
)

type Traceable struct {
	span opentracing.Span
}

func (t *Traceable) StartSpan(tracer opentracing.Tracer, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	t.span = tracer.StartSpan(spanName, opts...)
	return t.span
}
func (t *Traceable) FinishSpan() {
	if t == nil || t.span == nil {
		return
	}
	t.span.Finish()
	jaegerSpan := t.span.(*jaeger.Span)
	spanDurationMetric.WithLabelValues(jaegerSpan.OperationName()).Observe(jaegerSpan.Duration().Seconds())
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
