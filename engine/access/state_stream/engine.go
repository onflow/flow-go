package state_stream

import (
	"fmt"
	"net"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	lru "github.com/hashicorp/golang-lru"
	access "github.com/onflow/flow/protobuf/go/flow/executiondata"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	ListenAddr              string
	MaxExecutionDataMsgSize uint // in bytes
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

	execDataBroadcaster *engine.Broadcaster
	execDataCache       *lru.Cache
	latestExecDataCache *LatestEntityIDCache

	stateStreamGrpcAddress net.Addr
}

// New returns a new ingress server.
func NewEng(
	config Config,
	execDataStore execution_data.ExecutionDataStore,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	log zerolog.Logger,
	chainID flow.ChainID,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, GetExecutionDataByBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, GetExecutionDataByBlockID->10
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

	execDataCache, err := lru.New(DefaultCacheSize)
	if err != nil {
		return nil, fmt.Errorf("could not create cache: %w", err)
	}

	latestExecDataCache := NewLatestEntityIDCache()
	broadcaster := engine.NewBroadcaster()

	backend, err := New(logger, headers, seals, results, execDataStore, execDataCache, broadcaster, latestExecDataCache)
	if err != nil {
		return nil, fmt.Errorf("could not create state stream backend: %w", err)
	}

	handler := NewHandler(backend, chainID.Chain())

	e := &Engine{
		log:                 logger,
		backend:             backend,
		server:              server,
		chain:               chainID.Chain(),
		config:              config,
		handler:             handler,
		execDataBroadcaster: broadcaster,
		execDataCache:       execDataCache,
		latestExecDataCache: latestExecDataCache,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.serve).
		Build()
	access.RegisterExecutionDataAPIServer(e.server, e.handler)

	return e, nil
}

func (e *Engine) OnExecutionData(executionData *execution_data.BlockExecutionData) {
	e.log.Trace().Msgf("received execution data %v", executionData.BlockID)
	_ = e.execDataCache.Add(executionData.BlockID, executionData)
	e.latestExecDataCache.Set(executionData.BlockID)
	e.execDataBroadcaster.Publish()
	e.log.Trace().Msg("sent broadcast notification")
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

type LatestEntityIDCache struct {
	mu sync.RWMutex
	id flow.Identifier
}

func NewLatestEntityIDCache() *LatestEntityIDCache {
	return &LatestEntityIDCache{}
}

func (c *LatestEntityIDCache) Get() flow.Identifier {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

func (c *LatestEntityIDCache) Set(id flow.Identifier) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id = id
}
