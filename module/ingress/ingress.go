// Package ingress implements accepting transactions into the system.
// It implements a subset of the Observation API.
package ingress

import (
	"context"
	"fmt"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/network"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr string
}

// Ingress implements a gRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Ingress struct {
	unit    *engine.Unit
	logger  zerolog.Logger
	handler *handler     // the gRPC service implementation
	server  *grpc.Server // the gRPC server
	config  Config
}

// New returns a new ingress server.
func New(config Config, e *ingest.Engine) *Ingress {
	ingress := &Ingress{
		unit: engine.NewUnit(),
		handler: &handler{
			UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
			engine:                       e,
		},
		server: grpc.NewServer(),
		config: config,
	}

	access.RegisterAccessAPIServer(ingress.server, ingress.handler)

	return ingress
}

// Ready returns a ready channel that is closed once the module has fully
// started. The ingress module is ready when the gRPC server has successfully
// started.
func (i *Ingress) Ready() <-chan struct{} {
	i.unit.Launch(i.serve)
	return i.unit.Ready()
}

// Done returns a done channel that is closed once the module has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (i *Ingress) Done() <-chan struct{} {
	return i.unit.Done(i.server.GracefulStop)
}

// serve starts the gRPC server .
//
// When this function returns, the server is considered ready.
func (i *Ingress) serve() {

	log := i.logger.With().Str("module", "ingress").Logger()

	log.Info().Msgf("starting server on address %s", i.config.ListenAddr)

	l, err := net.Listen("tcp", i.config.ListenAddr)
	if err != nil {
		log.Err(err).Msg("failed to start server")
		return
	}

	err = i.server.Serve(l)
	if err != nil {
		log.Err(err).Msg("fatal error in server")
	}
}

// handler implements a subset of the Observation API.
type handler struct {
	access.UnimplementedAccessAPIServer
	engine network.Engine
}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

// SendTransaction accepts new transactions and inputs them to the ingress
// engine for validation and routing.
func (h *handler) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	err = h.engine.ProcessLocal(&tx)
	if err != nil {
		return nil, err
	}

	txID := tx.ID()

	return &access.SendTransactionResponse{Id: txID[:]}, nil
}
