package state_stream

import (
	"fmt"
	"net"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	logging "github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr              string
	MaxExecutionDataMsgSize int  // in bytes
	RpcMetricsEnabled       bool // enable GRPC metrics
}

// Engine exposes the server with the state stream API.
// By default, this engine is not enabled.
// In order to run this engine a port for the GRPC server to be served on should be specified in the run config.
type Engine struct {
	*component.ComponentManager
	log     zerolog.Logger
	backend *StateStreamBackend
	server  *grpc.Server
	config  Config
	chain   flow.Chain
	handler *Handler

	stateStreamGrpcAddress net.Addr
}

// New returns a new ingress server.
func NewEng(
	config Config,
	bs blobs.Blobstore,
	serializer execution_data.Serializer,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	log zerolog.Logger,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, GetExecutionDataByBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, GetExecutionDataByBlockID->10
) *Engine {
	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxExecutionDataMsgSize),
		grpc.MaxSendMsgSize(config.MaxExecutionDataMsgSize),
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

	// add the logging interceptor, ensure it is innermost wrapper
	interceptors = append(interceptors, logging.LoggingInterceptor(log)...)

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	grpcOpts = append(grpcOpts, chainedInterceptors)

	server := grpc.NewServer(grpcOpts...)

	execDataStore := execution_data.NewExecutionDataStore(bs, serializer)

	backend := New(headers, seals, results, execDataStore)

	e := &Engine{
		log:     log.With().Str("engine", "state_stream_rpc").Logger(),
		backend: backend,
		server:  server,
		chain:   chainID.Chain(),
		config:  config,
		handler: NewHandler(backend, chainID.Chain()),
	}

	componentBuilder := component.NewComponentManagerBuilder()
	componentBuilder.AddWorker(e.serve)
	e.ComponentManager = component.NewComponentManagerBuilder().Build()
	access.RegisterExecutionDataAPIServer(e.server, e.handler)

	return e
}

// serve starts the gRPC server.
// When this function returns, the server is considered ready.
func (e *Engine) serve(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	e.log.Info().Str("state_stream_address", e.config.ListenAddr).Msg("starting grpc server on address")
	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		ctx.Throw(fmt.Errorf("error starting grpc server: %w", err))
	}

	e.stateStreamGrpcAddress = l.Addr()
	e.log.Debug().Str("state_stream_address", e.stateStreamGrpcAddress.String()).Msg("listening on port")

	err = e.server.Serve(l) // blocking call
	if err != nil {
		ctx.Throw(fmt.Errorf("error trying to serve grpc server: %w", err))
	}

	select {
	case <-ctx.Done():
		e.server.GracefulStop()
	default:
	}
}
