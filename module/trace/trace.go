package trace

import (
	"io"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	config "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

// Tracer
type Tracer struct {
	opentracing.Tracer
	closer io.Closer
	log    zerolog.Logger
}

// NewTracer creates a new tracer
func NewTracer(log zerolog.Logger, service string) (*Tracer, error) {
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}
	metricsFactory := prometheus.New()
	tracer, closer, err := cfg.New(service, config.Metrics(metricsFactory))
	if err != nil {
		return nil, err
	}
	t := &Tracer{
		Tracer: tracer,
		closer: closer,
		log:    log,
	}
	return t, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (t *Tracer) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (t *Tracer) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		t.closer.Close()
		close(done)
	}()
	return done
}
