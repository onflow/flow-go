package state_stream

import (
	"net"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/grpcutils"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr        string
	MaxMsgSize        int  // in bytes
	RpcMetricsEnabled bool // enable GRPC metrics
}

// Engine exposes the server with a simplified version of the Access API.
// An unsecured GRPC server (default port 9000), a secure GRPC server (default port 9001) and an HTTP Web proxy (default
// port 8000) are brought up.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	backend *StateStreamBackend
	server  *grpc.Server
	config  Config
	chain   flow.Chain
	handler *Handler

	addrLock               sync.RWMutex
	stateStreamGrpcAddress net.Addr
}

// New returns a new ingress server.
func NewEng(
	config Config,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	execDownloader execution_data.Downloader,
	log zerolog.Logger,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, ExecuteScriptAtBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, ExecuteScriptAtBlockID->10
) *Engine {
	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	}

	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// if rpc metrics is enabled, add the grpc metrics interceptor as a server option
	if config.RpcMetricsEnabled {
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)
	}

	if len(apiRatelimits) > 0 {
		// create a rate limit interceptor
		rateLimitInterceptor := rpc.NewRateLimiterInterceptor(log, apiRatelimits, apiBurstLimits).UnaryServerInterceptor
		// append the rate limit interceptor to the list of interceptors
		interceptors = append(interceptors, rateLimitInterceptor)
	}

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	grpcOpts = append(grpcOpts, chainedInterceptors)

	server := grpc.NewServer(grpcOpts...)

	backend := New(headers, seals, results, execDownloader)

	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log.With().Str("engine", "state_stream_rpc").Logger(),
		backend: backend,
		server:  server,
		chain:   chainID.Chain(),
		config:  config,
		handler: NewHandler(backend, chainID.Chain()),
	}

	access.RegisterExecutionDataAPIServer(e.server, e.handler)

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
	e.log.Info().Str("state_stream_address", e.config.ListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		e.log.Fatal().Err(err).Msg("failed to start server")
		return
	}

	e.addrLock.Lock()
	e.stateStreamGrpcAddress = l.Addr()
	e.addrLock.Unlock()

	e.log.Debug().Str("state_stream_address", e.stateStreamGrpcAddress.String()).Msg("listening on port")

	err = e.server.Serve(l) // blocking call
	if err != nil {
		e.log.Fatal().Err(err).Msg("fatal error in secure grpc server")
	}
}
