package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	lru "github.com/hashicorp/golang-lru"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Config defines the configurable options for the access node server
// A secure GRPC server here implies a server that presents a self-signed TLS certificate and a client that authenticates
// the server via a pre-shared public key
type Config struct {
	UnsecureGRPCListenAddr    string                           // the non-secure GRPC server address as ip:port
	SecureGRPCListenAddr      string                           // the secure GRPC server address as ip:port
	TransportCredentials      credentials.TransportCredentials // the secure GRPC credentials
	HTTPListenAddr            string                           // the HTTP web proxy address as ip:port
	RESTListenAddr            string                           // the REST server address as ip:port (if empty the REST server will not be started)
	CollectionAddr            string                           // the address of the upstream collection node
	HistoricalAccessAddrs     string                           // the list of all access nodes from previous spork
	MaxMsgSize                uint                             // GRPC max message size
	ExecutionClientTimeout    time.Duration                    // execution API GRPC client timeout
	CollectionClientTimeout   time.Duration                    // collection API GRPC client timeout
	ConnectionPoolSize        uint                             // size of the cache for storing collection and execution connections
	MaxHeightRange            uint                             // max size of height range requests
	PreferredExecutionNodeIDs []string                         // preferred list of upstream execution node IDs
	FixedExecutionNodeIDs     []string                         // fixed list of execution node IDs to choose from if no node node ID can be chosen from the PreferredExecutionNodeIDs
	ArchiveAddressList        []string                         // the archive node address list to send script executions. when configured, script executions will be all sent to the archive node
}

// Engine exposes the server with a simplified version of the Access API.
// An unsecured GRPC server (default port 9000), a secure GRPC server (default port 9001) and an HTTP Web proxy (default
// port 8000) are brought up.
type Engine struct {
	component.Component

	finalizedHeaderCacheActor *events.FinalizationActor // consumes events to populate the finalized header cache
	backendNotifierActor      *events.FinalizationActor // consumes events to notify the backend of finalized heights
	finalizedHeaderCache      *events.FinalizedHeaderCache

	log                zerolog.Logger
	backend            *backend.Backend // the gRPC service implementation
	unsecureGrpcServer *grpc.Server     // the unsecure gRPC server
	secureGrpcServer   *grpc.Server     // the secure gRPC server
	httpServer         *http.Server
	restServer         *http.Server
	config             Config
	chain              flow.Chain

	addrLock            sync.RWMutex
	unsecureGrpcAddress net.Addr
	secureGrpcAddress   net.Addr
	restAPIAddress      net.Addr
}

// NewBuilder returns a new RPC engine builder.
func NewBuilder(log zerolog.Logger,
	state protocol.State,
	config Config,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	chainID flow.ChainID,
	transactionMetrics module.TransactionMetrics,
	accessMetrics module.AccessMetrics,
	collectionGRPCPort uint,
	executionGRPCPort uint,
	retryEnabled bool,
	rpcMetricsEnabled bool,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the Access API e.g. Ping->100, GetTransaction->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the Access API e.g. Ping->50, GetTransaction->10
	me module.Local,
) (*RPCEngineBuilder, error) {

	log = log.With().Str("engine", "rpc").Logger()

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(config.MaxMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxMsgSize)),
	}

	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// if rpc metrics is enabled, first create the grpc metrics interceptor
	if rpcMetricsEnabled {
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

	// create an unsecured grpc server
	unsecureGrpcServer := grpc.NewServer(grpcOpts...)

	// create a secure server by using the secure grpc credentials that are passed in as part of config
	grpcOpts = append(grpcOpts, grpc.Creds(config.TransportCredentials))
	secureGrpcServer := grpc.NewServer(grpcOpts...)

	// wrap the unsecured server with an HTTP proxy server to serve HTTP clients
	httpServer := newHTTPProxyServer(unsecureGrpcServer)

	var cache *lru.Cache
	cacheSize := config.ConnectionPoolSize
	if cacheSize > 0 {
		// TODO: remove this fallback after fixing issues with evictions
		// It was observed that evictions cause connection errors for in flight requests. This works around
		// the issue by forcing hte pool size to be greater than the number of ENs + LNs
		if cacheSize < backend.DefaultConnectionPoolSize {
			log.Warn().Msg("connection pool size below threshold, setting pool size to default value ")
			cacheSize = backend.DefaultConnectionPoolSize
		}
		var err error
		cache, err = lru.NewWithEvict(int(cacheSize), func(_, evictedValue interface{}) {
			store := evictedValue.(*backend.CachedClient)
			store.Close()
			log.Debug().Str("grpc_conn_evicted", store.Address).Msg("closing grpc connection evicted from pool")
			if accessMetrics != nil {
				accessMetrics.ConnectionFromPoolEvicted()
			}
		})
		if err != nil {
			return nil, fmt.Errorf("could not initialize connection pool cache: %w", err)
		}
	}

	connectionFactory := &backend.ConnectionFactoryImpl{
		CollectionGRPCPort:        collectionGRPCPort,
		ExecutionGRPCPort:         executionGRPCPort,
		CollectionNodeGRPCTimeout: config.CollectionClientTimeout,
		ExecutionNodeGRPCTimeout:  config.ExecutionClientTimeout,
		ConnectionsCache:          cache,
		CacheSize:                 cacheSize,
		MaxMsgSize:                config.MaxMsgSize,
		AccessMetrics:             accessMetrics,
		Log:                       log,
	}

	backend := backend.New(state,
		collectionRPC,
		historicalAccessNodes,
		blocks,
		headers,
		collections,
		transactions,
		executionReceipts,
		executionResults,
		chainID,
		transactionMetrics,
		connectionFactory,
		retryEnabled,
		config.MaxHeightRange,
		config.PreferredExecutionNodeIDs,
		config.FixedExecutionNodeIDs,
		log,
		backend.DefaultSnapshotHistoryLimit,
		config.ArchiveAddressList,
	)

	finalizedCache, finalizedCacheWorker, err := events.NewFinalizedHeaderCache(state)
	if err != nil {
		return nil, fmt.Errorf("could not create header cache: %w", err)
	}

	eng := &Engine{
		finalizedHeaderCache:      finalizedCache,
		finalizedHeaderCacheActor: finalizedCache.FinalizationActor,
		log:                       log,
		backend:                   backend,
		unsecureGrpcServer:        unsecureGrpcServer,
		secureGrpcServer:          secureGrpcServer,
		httpServer:                httpServer,
		config:                    config,
		chain:                     chainID.Chain(),
	}
	backendNotifierActor, backendNotifierWorker := events.NewFinalizationActor(eng.notifyBackendOnBlockFinalized)
	eng.backendNotifierActor = backendNotifierActor

	eng.Component = component.NewComponentManagerBuilder().
		AddWorker(eng.serveUnsecureGRPCWorker).
		AddWorker(eng.serveSecureGRPCWorker).
		AddWorker(eng.serveGRPCWebProxyWorker).
		AddWorker(eng.serveREST).
		AddWorker(finalizedCacheWorker).
		AddWorker(backendNotifierWorker).
		AddWorker(eng.shutdownWorker).
		Build()

	builder := NewRPCEngineBuilder(eng, me, finalizedCache)
	if rpcMetricsEnabled {
		builder.WithMetrics()
	}

	return builder, nil
}

// shutdownWorker is a worker routine which shuts down all servers when the context is cancelled.
func (e *Engine) shutdownWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	<-ctx.Done()
	e.shutdown()
}

// shutdown sequentially shuts down all servers managed by this engine.
// Errors which occur while shutting down a server are logged and otherwise ignored.
func (e *Engine) shutdown() {
	// use unbounded context, rely on shutdown logic to have timeout
	ctx := context.Background()

	e.unsecureGrpcServer.GracefulStop()
	e.secureGrpcServer.GracefulStop()
	err := e.httpServer.Shutdown(ctx)
	if err != nil {
		e.log.Error().Err(err).Msg("error stopping http server")
	}
	if e.restServer != nil {
		err := e.restServer.Shutdown(ctx)
		if err != nil {
			e.log.Error().Err(err).Msg("error stopping http REST server")
		}
	}
}

// OnBlockFinalized responds to block finalization events.
func (e *Engine) OnBlockFinalized(block *model.Block) {
	e.finalizedHeaderCacheActor.OnBlockFinalized(block)
	e.backendNotifierActor.OnBlockFinalized(block)
}

// notifyBackendOnBlockFinalized is invoked by the FinalizationActor when a new block is finalized.
// It notifies the backend of the newly finalized block.
func (e *Engine) notifyBackendOnBlockFinalized(_ *model.Block) error {
	finalizedHeader := e.finalizedHeaderCache.Get()
	e.backend.NotifyFinalizedBlockHeight(finalizedHeader.Height)
	return nil
}

// UnsecureGRPCAddress returns the listen address of the unsecure GRPC server.
// Guaranteed to be non-nil after Engine.Ready is closed.
func (e *Engine) UnsecureGRPCAddress() net.Addr {
	e.addrLock.RLock()
	defer e.addrLock.RUnlock()
	return e.unsecureGrpcAddress
}

// SecureGRPCAddress returns the listen address of the secure GRPC server.
// Guaranteed to be non-nil after Engine.Ready is closed.
func (e *Engine) SecureGRPCAddress() net.Addr {
	e.addrLock.RLock()
	defer e.addrLock.RUnlock()
	return e.secureGrpcAddress
}

// RestApiAddress returns the listen address of the REST API server.
// Guaranteed to be non-nil after Engine.Ready is closed.
func (e *Engine) RestApiAddress() net.Addr {
	e.addrLock.RLock()
	defer e.addrLock.RUnlock()
	return e.restAPIAddress
}

// serveUnsecureGRPCWorker is a worker routine which starts the unsecure gRPC server.
// The ready callback is called after the server address is bound and set.
func (e *Engine) serveUnsecureGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.log.Info().Str("grpc_address", e.config.UnsecureGRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.UnsecureGRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		ctx.Throw(err)
		return
	}

	// save the actual address on which we are listening (may be different from e.config.UnsecureGRPCListenAddr if not port
	// was specified)
	e.addrLock.Lock()
	e.unsecureGrpcAddress = l.Addr()
	e.addrLock.Unlock()
	e.log.Debug().Str("unsecure_grpc_address", e.unsecureGrpcAddress.String()).Msg("listening on port")
	ready()

	err = e.unsecureGrpcServer.Serve(l) // blocking call
	if err != nil {
		e.log.Err(err).Msg("fatal error in unsecure grpc server")
		ctx.Throw(err)
	}
}

// serveSecureGRPCWorker is a worker routine which starts the secure gRPC server.
// The ready callback is called after the server address is bound and set.
func (e *Engine) serveSecureGRPCWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	e.log.Info().Str("secure_grpc_address", e.config.SecureGRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.SecureGRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		ctx.Throw(err)
		return
	}

	e.addrLock.Lock()
	e.secureGrpcAddress = l.Addr()
	e.addrLock.Unlock()

	e.log.Debug().Str("secure_grpc_address", e.secureGrpcAddress.String()).Msg("listening on port")
	ready()

	err = e.secureGrpcServer.Serve(l) // blocking call
	if err != nil {
		e.log.Err(err).Msg("fatal error in secure grpc server")
		ctx.Throw(err)
	}
}

// serveGRPCWebProxyWorker is a worker routine which starts the gRPC web proxy server.
func (e *Engine) serveGRPCWebProxyWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	log := e.log.With().Str("http_proxy_address", e.config.HTTPListenAddr).Logger()
	log.Info().Msg("starting http proxy server on address")

	l, err := net.Listen("tcp", e.config.HTTPListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc web proxy server")
		ctx.Throw(err)
		return
	}
	ready()

	err = e.httpServer.Serve(l) // blocking call
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		log.Err(err).Msg("fatal error in grpc web proxy server")
		ctx.Throw(err)
	}
}

// serveREST is a worker routine which starts the HTTP REST server.
// The ready callback is called after the server address is bound and set.
func (e *Engine) serveREST(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if e.config.RESTListenAddr == "" {
		e.log.Debug().Msg("no REST API address specified - not starting the server")
		ready()
		return
	}

	e.log.Info().Str("rest_api_address", e.config.RESTListenAddr).Msg("starting REST server on address")

	r, err := rest.NewServer(e.backend, e.config.RESTListenAddr, e.log, e.chain)
	if err != nil {
		e.log.Err(err).Msg("failed to initialize the REST server")
		ctx.Throw(err)
		return
	}
	e.restServer = r

	l, err := net.Listen("tcp", e.config.RESTListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the REST server")
		ctx.Throw(err)
		return
	}

	e.addrLock.Lock()
	e.restAPIAddress = l.Addr()
	e.addrLock.Unlock()

	e.log.Debug().Str("rest_api_address", e.restAPIAddress.String()).Msg("listening on port")
	ready()

	err = e.restServer.Serve(l) // blocking call
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		e.log.Err(err).Msg("fatal error in REST server")
		ctx.Throw(err)
	}
}
