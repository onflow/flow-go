// Package rpc implements accepting transactions into the system.
// It implements a subset of the Observation API.
package rpc

import (
	"context"
	"fmt"
	"net"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// Backend defines the core functionality required by the RPC API.
type Backend interface {
	// ProcessTransaction handles validating and ingesting a new transaction,
	// ultimately for inclusion in a future collection.
	ProcessTransaction(*flow.TransactionBody) error
}

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr        string
	MaxMsgSize        int  // in bytes
	RpcMetricsEnabled bool // enable GRPC metrics
}

// Engine implements a gRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	handler *handler     // the gRPC service implementation
	server  *grpc.Server // the gRPC server
	config  Config
}

// New returns a new ingress server.
func New(config Config, backend Backend, log zerolog.Logger, chainID flow.ChainID) *Engine {
	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	}

	// if rpc metrics is enabled, add the grpc metrics interceptor as a server option
	if config.RpcMetricsEnabled {
		grpcOpts = append(grpcOpts, grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor))
	}

	server := grpc.NewServer(grpcOpts...)

	e := &Engine{
		unit: engine.NewUnit(),
		log:  log.With().Str("engine", "collection_rpc").Logger(),
		handler: &handler{
			UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
			backend:                      backend,
			chainID:                      chainID,
		},
		server: server,
		config: config,
	}

	if config.RpcMetricsEnabled {
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(server)
	}

	access.RegisterAccessAPIServer(e.server, e.handler)

	return e
}

// Ready returns a ready channel that is closed once the module has fully
// started. The ingress module is ready when the gRPC server has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.serve)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the module has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(e.server.GracefulStop)
}

// serve starts the gRPC server .
//
// When this function returns, the server is considered ready.
func (e *Engine) serve() {

	e.log.Info().Msgf("starting server on address %s", e.config.ListenAddr)

	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		e.log.Fatal().Err(err).Msg("failed to start server")
		return
	}

	err = e.server.Serve(l)
	if err != nil {
		e.log.Error().Err(err).Msg("fatal error in server")
	}
}

// handler implements a subset of the Observation API.
type handler struct {
	access.UnimplementedAccessAPIServer
	backend Backend
	chainID flow.ChainID
}

// Ping responds to requests when the server is up.
func (h *handler) Ping(_ context.Context, _ *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

// SendTransaction accepts new transactions and inputs them to the ingress
// engine for validation and routing.
func (h *handler) SendTransaction(_ context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	tx, err := convert.MessageToTransaction(req.Transaction, h.chainID.Chain())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	err = h.backend.ProcessTransaction(&tx)
	if err != nil {
		return nil, err
	}

	txID := tx.ID()

	return &access.SendTransactionResponse{Id: txID[:]}, nil
}
