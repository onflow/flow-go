// Package rpc implements accepting transactions into the system.
// It implements a subset of the Observation API.
package rpc

import (
	"context"
	"fmt"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // required for gRPC compression
	"google.golang.org/grpc/status"

	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate" // required for gRPC compression
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"  // required for gRPC compression

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/grpcserver"
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
	MaxMsgSize        uint // in bytes
	RpcMetricsEnabled bool // enable GRPC metrics
}

// Engine implements a gRPC server with a simplified version of the Observation
// API to enable receiving transactions into the system.
type Engine struct {
	component.Component
	server  *grpcserver.GrpcServer
	handler *handler
	log     zerolog.Logger
}

// New returns a new ingress server.
func New(
	log zerolog.Logger,
	server *grpcserver.GrpcServer,
	config Config,
	backend Backend,
	chainID flow.ChainID,
) *Engine {
	log = log.With().Str("engine", "collection_rpc").Logger()

	handler := &handler{
		UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
		backend:                      backend,
		chainID:                      chainID,
	}

	if config.RpcMetricsEnabled {
		grpc_prometheus.EnableHandlingTimeHistogram()
		server.RegisterService(func(s *grpc.Server) {
			grpc_prometheus.Register(s)
		})
	}

	server.RegisterService(func(s *grpc.Server) {
		access.RegisterAccessAPIServer(s, handler)
	})

	return &Engine{
		Component: server,
		server:    server,
		handler:   handler,
		log:       log,
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
	if engine.IsInvalidInputError(err) {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, err
	}

	txID := tx.ID()

	return &access.SendTransactionResponse{Id: txID[:]}, nil
}
