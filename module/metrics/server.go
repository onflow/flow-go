package metrics

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// Server is the http server that will be serving the /metrics request for prometheus
type Server struct {
	server *http.Server
	log    zerolog.Logger
}

// NewServer creates a new server that will start on the specified port,
// and responds to only the `/metrics` endpoint
func NewServer(log zerolog.Logger, port uint, enableProfilerEndpoint bool) *Server {
	addr := ":" + strconv.Itoa(int(port))

	mux := http.NewServeMux()
	endpoint := "/metrics"
	mux.Handle(endpoint, promhttp.Handler())
	log.Info().Str("address", addr).Str("endpoint", endpoint).Msg("metrics server started")
	if enableProfilerEndpoint {
		mux.Handle("/debug/pprof/", http.DefaultServeMux)
	}

	m := &Server{
		server: &http.Server{Addr: addr, Handler: mux},
		log:    log,
	}

	return m
}

// Ready returns a channel that will close when the network stack is ready.
func (m *Server) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		if err := m.server.ListenAndServe(); err != nil {
			// http.ErrServerClosed is returned when Close or Shutdown is called
			// we don't consider this an error, so print this with debug level instead
			if errors.Is(err, http.ErrServerClosed) {
				m.log.Debug().Err(err).Msg("metrics server shutdown")
			} else {
				m.log.Err(err).Msg("error shutting down metrics server")
			}
		}
	}()
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (m *Server) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = m.server.Shutdown(ctx)
		cancel()
		close(done)
	}()
	return done
}
