package state_stream

import (
	"fmt"
	"net"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	EventFilterConfig

	// ListenAddr is the address the GRPC server will listen on as host:port
	ListenAddr string

	// MaxExecutionDataMsgSize is the max message size for block execution data API
	MaxExecutionDataMsgSize uint

	// RpcMetricsEnabled specifies whether to enable the GRPC metrics
	RpcMetricsEnabled bool

	// MaxGlobalStreams defines the global max number of streams that can be open at the same time.
	MaxGlobalStreams uint32

	// ExecutionDataCacheSize is the max number of objects for the execution data cache.
	ExecutionDataCacheSize uint32

	// ClientSendTimeout is the timeout for sending a message to the client. After the timeout,
	// the stream is closed with an error.
	ClientSendTimeout time.Duration

	// ClientSendBufferSize is the size of the response buffer for sending messages to the client.
	ClientSendBufferSize uint
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

	execDataBroadcaster *engine.Broadcaster
	execDataCache       *herocache.BlockExecutionData

	stateStreamGrpcAddress net.Addr
}

// NewEng returns a new ingress server.
func NewEng(
	log zerolog.Logger,
	config Config,
	execDataStore execution_data.ExecutionDataStore,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, GetExecutionDataByBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, GetExecutionDataByBlockID->10
	heroCacheMetrics module.HeroCacheMetrics,
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

	execDataCache := herocache.NewBlockExecutionData(config.ExecutionDataCacheSize, logger, heroCacheMetrics)

	broadcaster := engine.NewBroadcaster()

	backend, err := New(logger, config, state, headers, seals, results, execDataStore, execDataCache, broadcaster)
	if err != nil {
		return nil, fmt.Errorf("could not create state stream backend: %w", err)
	}

	e := &Engine{
		log:                 logger,
		backend:             backend,
		server:              server,
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
func (e *Engine) OnExecutionData(executionData *execution_data.BlockExecutionDataEntity) {
	lg := e.log.With().Hex("block_id", logging.ID(executionData.BlockID)).Logger()

	lg.Trace().Msg("received execution data")

	if ok := e.execDataCache.Add(executionData); !ok {
		lg.Warn().Msg("failed to add execution data to cache")
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
