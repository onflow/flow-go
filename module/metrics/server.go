package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// metricsServerShutdownTimeout is the time to wait for the server to shut down gracefully
const metricsServerShutdownTimeout = 5 * time.Second

// Server is the http server that will be serving the /metrics request for prometheus
type Server struct {
	server *http.Server
	log    zerolog.Logger

	startupCompleted chan struct{}
}

var _ component.Component = (*Server)(nil)

// NewServer creates a new server that will start on the specified port,
// and responds to only the `/metrics` endpoint
func NewServer(log zerolog.Logger, port uint) *Server {
	addr := ":" + strconv.Itoa(int(port))

	mux := http.NewServeMux()
	endpoint := "/metrics"
	mux.Handle(endpoint, promhttp.Handler())
	log.Info().Str("address", addr).Str("endpoint", endpoint).Msg("metrics server started")

	m := &Server{
		server:           &http.Server{Addr: addr, Handler: mux},
		log:              log,
		startupCompleted: make(chan struct{}),
	}

	return m
}

func (m *Server) Start(signalerContext irrecoverable.SignalerContext) {
	defer close(m.startupCompleted)

	// pass the signaler context to the server so that the signaler context
	// can control the server's lifetime
	m.server.BaseContext = func(_ net.Listener) context.Context {
		return signalerContext
	}

	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			// http.ErrServerClosed is returned when Close or Shutdown is called
			// we don't consider this an error, so print this with debug level instead
			if errors.Is(err, http.ErrServerClosed) {
				m.log.Debug().Err(err).Msg("metrics server shutdown")
			} else {
				m.log.Err(err).Msg("error running metrics server")
			}
		}
	}()
}

// Ready returns a channel that will close when the network stack is ready.
func (m *Server) Ready() <-chan struct{} {
	ready := make(chan struct{})

	go func() {
		<-m.startupCompleted
		close(ready)
	}()

	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (m *Server) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		<-m.startupCompleted
		defer close(done)

		ctx, cancel := context.WithTimeout(context.Background(), metricsServerShutdownTimeout)
		defer cancel()

		// shutdown the server gracefully
		err := m.server.Shutdown(ctx)
		if err == nil {
			m.log.Info().Msg("metrics server graceful shutdown completed")
			return
		}

		if errors.Is(err, ctx.Err()) {
			m.log.Warn().Msg("metrics server graceful shutdown timed out")
			// shutdown the server forcefully
			err := m.server.Close()
			if err != nil {
				m.log.Err(err).Msg("error closing metrics server")
			}
		} else {
			m.log.Err(err).Msg("error shutting down metrics server")
		}
	}()
	return done
}
