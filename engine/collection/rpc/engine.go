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
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"

	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate"
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	cm *component.ComponentManager

	log     zerolog.Logger
	handler *handler
	server  *grpc.Server
	config  Config
}

// New returns a new ingress server.
func New(
	config Config,
	backend Backend,
	log zerolog.Logger,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, ExecuteScriptAtBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, ExecuteScriptAtBlockID->10
) *Engine {
	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(config.MaxMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxMsgSize)),
	}

	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// if rpc metrics is enabled, add the grpc metrics interceptor as a server option
	if config.RpcMetricsEnabled {
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)
	}

	if len(apiRatelimits) > 0 {
		// create a rate limit interceptor
		rateLimitInterceptor := grpcserver.NewRateLimiterInterceptor(log, apiRatelimits, apiBurstLimits).UnaryServerInterceptor
		// append the rate limit interceptor to the list of interceptors
		interceptors = append(interceptors, rateLimitInterceptor)
	}

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	grpcOpts = append(grpcOpts, chainedInterceptors)

	server := grpc.NewServer(grpcOpts...)

	e := &Engine{
		log: log.With().Str("engine", "collection_rpc").Logger(),
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

	e.cm = component.NewComponentManagerBuilder().
		AddWorker(e.serveGRPCWorker).
		AddWorker(e.shutdownWorker).
		Build()
	e.Component = e.cm

	return e
}

func (e *Engine) serveGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.log.Info().Str("listen_addr", e.config.ListenAddr).Msg("starting server on address")

	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start server")
		ctx.Throw(err)
		return
	}
	ready()

	if err := e.server.Serve(l); err != nil {
		e.log.Error().Err(err).Msg("fatal error in server")
		ctx.Throw(err)
	}
}

func (e *Engine) shutdownWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	<-ctx.Done()
	e.server.GracefulStop()
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
