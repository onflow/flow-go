package trace

import (
	"io"

	"github.com/dapperlabs/flow-go/model/flow"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"
	config "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

// OpenTracer
type OpenTracer struct {
	opentracing.Tracer
	closer    io.Closer
	log       zerolog.Logger
	openSpans map[flow.Identifier]opentracing.Span
}

type traceLogger struct {
	zerolog.Logger
}

func (t traceLogger) Error(msg string) {
	t.Logger.Error().Msg(msg)
}

// Infof logs a message at info priority
func (t traceLogger) Infof(msg string, args ...interface{}) {
	t.Debug().Msgf(msg, args...)
}

// NewTracer creates a new tracer
func NewTracer(log zerolog.Logger, service string) (Tracer, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}
	metricsFactory := prometheus.New()
	log.Debug().Msgf("%+v", cfg)
	tracer, closer, err := cfg.New(service, config.Metrics(metricsFactory), config.Logger(traceLogger{log}))
	if err != nil {
		return nil, err
	}
	t := &OpenTracer{
		Tracer:    tracer,
		closer:    closer,
		log:       log,
		openSpans: map[flow.Identifier]opentracing.Span{},
	}
	return t, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (t *OpenTracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *OpenTracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		t.closer.Close()
		close(done)
	}()
	return done
}

func (t *OpenTracer) StartSpan(entity flow.Identifier, spanName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	t.openSpans[entity] = t.Tracer.StartSpan(spanName, opts...)
	return t.openSpans[entity]
}
func (t *OpenTracer) FinishSpan(entity flow.Identifier) {
	span, ok := t.openSpans[entity]
	if ok {
		span.Finish()
		jaegerSpan := span.(*jaeger.Span)
		spanDurationMetric.WithLabelValues(jaegerSpan.OperationName()).Observe(jaegerSpan.Duration().Seconds())
	}
	delete(t.openSpans, entity)
}

func (t *OpenTracer) GetSpan(entity flow.Identifier) opentracing.Span {
	return t.openSpans[entity]
}
