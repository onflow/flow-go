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
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// metricsServerShutdownTimeout is the time to wait for the server to shut down gracefully
const metricsServerShutdownTimeout = 5 * time.Second

// Server is the http server that will be serving the /metrics request for prometheus
type Server struct {
	component.Component

	address string
	server  *http.Server
	log     zerolog.Logger
}

// NewServer creates a new server that will start on the specified port,
// and responds to only the `/metrics` endpoint
func NewServer(log zerolog.Logger, port uint) *Server {
	addr := ":" + strconv.Itoa(int(port))

	mux := http.NewServeMux()
	endpoint := "/metrics"
	mux.Handle(endpoint, promhttp.Handler())

	m := &Server{
		address: addr,
		server:  &http.Server{Addr: addr, Handler: mux},
		log:     log.With().Str("address", addr).Str("endpoint", endpoint).Logger(),
	}

	m.Component = component.NewComponentManagerBuilder().
		AddWorker(m.serve).
		AddWorker(m.shutdownOnContextDone).
		Build()

	return m
}

func (m *Server) serve(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	m.log.Info().Msg("starting metrics server on address")

	l, err := net.Listen("tcp", m.address)
	if err != nil {
		m.log.Err(err).Msg("failed to start the metrics server")
		ctx.Throw(err)
		return
	}

	ready()

	// pass the signaler context to the server so that the signaler context
	// can control the server's lifetime
	m.server.BaseContext = func(_ net.Listener) context.Context {
		return ctx
	}

	err = m.server.Serve(l) // blocking call
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		log.Err(err).Msg("fatal error in the metrics server")
		ctx.Throw(err)
	}
}

func (m *Server) shutdownOnContextDone(ictx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	<-ictx.Done()

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
}
