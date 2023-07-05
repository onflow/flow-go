package state_stream

import (
	"fmt"
	"net"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/state_stream"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"

	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
)

// Engine exposes the server with the state stream API.
// By default, this engine is not enabled.
// In order to run this engine a port for the GRPC server to be served on should be specified in the run config.
type Engine struct {
	*component.ComponentManager
	log     zerolog.Logger
	backend *state_stream.StateStreamBackend
	server  *grpc.Server
	config  state_stream.Config
	chain   flow.Chain
	handler *Handler

	execDataBroadcaster *engine.Broadcaster
	execDataCache       *cache.ExecutionDataCache
	headers             storage.Headers

	stateStreamGrpcAddress net.Addr
}

// NewEng returns a new ingress server.
func NewEng(
	log zerolog.Logger,
	config state_stream.Config,
	execDataCache *cache.ExecutionDataCache,
	headers storage.Headers,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, GetExecutionDataByBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, GetExecutionDataByBlockID->10
	backend *state_stream.StateStreamBackend,
	broadcaster *engine.Broadcaster,
) (*Engine, error) {
	logger := log.With().Str("engine", "state_stream_rpc").Logger()

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(config.MaxExecutionDataMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxExecutionDataMsgSize)),
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
	interceptors = append(interceptors, rpc.LoggingInterceptor(log)...)

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	grpcOpts = append(grpcOpts, chainedInterceptors)

	server := grpc.NewServer(grpcOpts...)

	e := &Engine{
		log:                 logger,
		backend:             backend,
		server:              server,
		headers:             headers,
		chain:               chainID.Chain(),
		config:              config,
		handler:             NewHandler(backend, chainID.Chain(), config.EventFilterConfig, config.MaxGlobalStreams),
		execDataBroadcaster: broadcaster,
		execDataCache:       execDataCache,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.serve).
		Build()

	access.RegisterExecutionDataAPIServer(e.server, e.handler)

	return e, nil
}

// OnExecutionData is called to notify the engine when a new execution data is received.
// The caller must guarantee that execution data is locally available for all blocks with
// heights between the initialBlockHeight provided during startup and the block height of
// the execution data provided.
func (e *Engine) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	lg := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()

	lg.Trace().Msg("received execution data")

	header, err := e.headers.ByBlockID(executionData.BlockID)
	if err != nil {
		// if the execution data is available, the block must be locally finalized
		lg.Fatal().Err(err).Msg("failed to get header for execution data")
		return
	}

	if ok := e.backend.SetHighestHeight(header.Height); !ok {
		// this means that the height was lower than the current highest height
		// OnExecutionData is guaranteed by the requester to be called in order, but may be called
		// multiple times for the same block.
		lg.Debug().Msg("execution data for block already received")
		return
	}

	e.execDataBroadcaster.Publish()
}

// serve starts the gRPC server.
// When this function returns, the server is considered ready.
func (e *Engine) serve(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.log.Info().Str("state_stream_address", e.config.ListenAddr).Msg("starting grpc server on address")
	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		ctx.Throw(fmt.Errorf("error starting grpc server: %w", err))
	}

	e.stateStreamGrpcAddress = l.Addr()
	e.log.Debug().Str("state_stream_address", e.stateStreamGrpcAddress.String()).Msg("listening on port")

	go func() {
		ready()
		err = e.server.Serve(l)
		if err != nil {
			ctx.Throw(fmt.Errorf("error trying to serve grpc server: %w", err))
		}
	}()

	<-ctx.Done()
	e.server.GracefulStop()
}
