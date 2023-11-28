package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/state_stream"
	statestreambackend "github.com/onflow/flow-go/engine/access/state_stream/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/module/grpcserver"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

// Config defines the configurable options for the access node server
// A secure GRPC server here implies a server that presents a self-signed TLS certificate and a client that authenticates
// the server via a pre-shared public key
type Config struct {
	UnsecureGRPCListenAddr string                           // the non-secure GRPC server address as ip:port
	SecureGRPCListenAddr   string                           // the secure GRPC server address as ip:port
	TransportCredentials   credentials.TransportCredentials // the secure GRPC credentials
	HTTPListenAddr         string                           // the HTTP web proxy address as ip:port
	CollectionAddr         string                           // the address of the upstream collection node
	HistoricalAccessAddrs  string                           // the list of all access nodes from previous spork

	BackendConfig  backend.Config // configurable options for creating Backend
	RestConfig     rest.Config    // the REST server configuration
	MaxMsgSize     uint           // GRPC max message size
	CompressorName string         // GRPC compressor name
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
	restCollector      module.RestMetrics
	backend            *backend.Backend       // the gRPC service implementation
	unsecureGrpcServer *grpcserver.GrpcServer // the unsecure gRPC server
	secureGrpcServer   *grpcserver.GrpcServer // the secure gRPC server
	httpServer         *http.Server
	restServer         *http.Server
	config             Config
	chain              flow.Chain

	restHandler access.API

	addrLock       sync.RWMutex
	restAPIAddress net.Addr

	stateStreamBackend state_stream.API
	stateStreamConfig  statestreambackend.Config
}
type Option func(*RPCEngineBuilder)

// NewBuilder returns a new RPC engine builder.
func NewBuilder(log zerolog.Logger,
	state protocol.State,
	config Config,
	chainID flow.ChainID,
	accessMetrics module.AccessMetrics,
	rpcMetricsEnabled bool,
	me module.Local,
	backend *backend.Backend,
	restHandler access.API,
	secureGrpcServer *grpcserver.GrpcServer,
	unsecureGrpcServer *grpcserver.GrpcServer,
	stateStreamBackend state_stream.API,
	stateStreamConfig statestreambackend.Config,
) (*RPCEngineBuilder, error) {
	log = log.With().Str("engine", "rpc").Logger()

	// wrap the unsecured server with an HTTP proxy server to serve HTTP clients
	httpServer := newHTTPProxyServer(unsecureGrpcServer.Server)

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
		restCollector:             accessMetrics,
		restHandler:               restHandler,
		stateStreamBackend:        stateStreamBackend,
		stateStreamConfig:         stateStreamConfig,
	}
	backendNotifierActor, backendNotifierWorker := events.NewFinalizationActor(eng.processOnFinalizedBlock)
	eng.backendNotifierActor = backendNotifierActor

	eng.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			<-secureGrpcServer.Done()
		}).
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			<-unsecureGrpcServer.Done()
		}).
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

// OnFinalizedBlock responds to block finalization events.
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	e.finalizedHeaderCacheActor.OnFinalizedBlock(block)
	e.backendNotifierActor.OnFinalizedBlock(block)
}

// processOnFinalizedBlock is invoked by the FinalizationActor when a new block is finalized.
// It informs the backend of the newly finalized block.
// The input to this callback is treated as trusted.
// No errors expected during normal operations.
func (e *Engine) processOnFinalizedBlock(_ *model.Block) error {
	finalizedHeader := e.finalizedHeaderCache.Get()
	return e.backend.ProcessFinalizedBlockHeight(finalizedHeader.Height)
}

// RestApiAddress returns the listen address of the REST API server.
// Guaranteed to be non-nil after Engine.Ready is closed.
func (e *Engine) RestApiAddress() net.Addr {
	e.addrLock.RLock()
	defer e.addrLock.RUnlock()
	return e.restAPIAddress
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
// Note: The original REST BaseContext is discarded, and the irrecoverable.SignalerContext is used for error handling.
func (e *Engine) serveREST(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if e.config.RestConfig.ListenAddress == "" {
		e.log.Debug().Msg("no REST API address specified - not starting the server")
		ready()
		return
	}

	e.log.Info().Str("rest_api_address", e.config.RestConfig.ListenAddress).Msg("starting REST server on address")

	r, err := rest.NewServer(e.restHandler, e.config.RestConfig, e.log, e.chain, e.restCollector, e.stateStreamBackend,
		e.stateStreamConfig)
	if err != nil {
		e.log.Err(err).Msg("failed to initialize the REST server")
		ctx.Throw(err)
		return
	}
	e.restServer = r

	// Set up the irrecoverable.SignalerContext for error handling in the REST server.
	e.restServer.BaseContext = func(_ net.Listener) context.Context {
		return irrecoverable.WithSignalerContext(ctx, ctx)
	}

	l, err := net.Listen("tcp", e.config.RestConfig.ListenAddress)
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
