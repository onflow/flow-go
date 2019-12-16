// Package ingress implements an API for accepting transactions into the
// network.
package ingress

import (
	"context"
	"fmt"
	"net"

	"github.com/dapperlabs/flow-go/engine"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/convert"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr string
}

// Ingress implements a GRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Ingress struct {
	logger  zerolog.Logger
	handler *handler     // the GRPC service implementation
	server  *grpc.Server // the GRPC server
	config  Config

	ready chan struct{} // indicates when the server is setup
	stop  chan struct{} // indicates that the server should stop
}

// New returns a new ingress server.
func New(config Config, engine *ingest.Engine) *Ingress {
	ingress := &Ingress{
		handler: &handler{
			engine: engine,
		},
		server: grpc.NewServer(),
		config: config,
		ready:  make(chan struct{}),
		stop:   make(chan struct{}),
	}

	observation.RegisterBasicObserveServiceServer(ingress.server, ingress.handler)

	return ingress
}

// Ready returns a ready channel that is closed once the module has fully
// started. The ingress module is ready when the GRPC server has successfully
// started.
func (i *Ingress) Ready() <-chan struct{} {
	return i.ready
}

// Done returns a done channel that is closed once the module has fully stopped.
// It sends a signal to stop the GRPC server, then closes the channel.
func (i Ingress) Done() <-chan struct{} {
	done := make(chan struct{})

	<-i.stop
	go func() {
		close(done)
	}()

	return done
}

// start starts the GRPC server and handles sending the ready signal and
// handling the stop signal.
func (i *Ingress) start() {
	log := i.logger.With().Str("module", "ingress").Logger()

	log.Info().Msgf("starting server on address %s", i.config.ListenAddr)

	l, err := net.Listen("tcp", i.config.ListenAddr)
	if err != nil {
		log.Err(err).Msg("failed to start server")
		return
	}

	// indicate that the server has started
	go func() {
		close(i.ready)
	}()

	// wait for a stop signal, then gracefully stop the server
	go func() {
		select {
		case <-i.stop:
			i.server.GracefulStop()
		}
	}()

	err = i.server.Serve(l)
	if err != nil {
		log.Err(err).Msg("fatal error in server")
	}
}

// handler implements the basic Observation API.
type handler struct {
	engine engine.Engine
}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	return &observation.PingResponse{}, nil
}

// SendTransaction accepts new transactions and inputs them to the ingress
// engine for validation and routing.
func (h *handler) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	h.engine.Submit(&tx)

	return &observation.SendTransactionResponse{Hash: tx.Hash()}, nil
}
