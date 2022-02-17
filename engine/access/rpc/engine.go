package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/grpcutils"
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
	MaxMsgSize                int                              // GRPC max message size
	ExecutionClientTimeout    time.Duration                    // execution API GRPC client timeout
	CollectionClientTimeout   time.Duration                    // collection API GRPC client timeout
	MaxHeightRange            uint                             // max size of height range requests
	PreferredExecutionNodeIDs []string                         // preferred list of upstream execution node IDs
	FixedExecutionNodeIDs     []string                         // fixed list of execution node IDs to choose from if no node node ID can be chosen from the PreferredExecutionNodeIDs
}

// Engine exposes the server with a simplified version of the Access API.
// An unsecured GRPC server (default port 9000), a secure GRPC server (default port 9001) and an HTTP Web proxy (default
// port 8000) are brought up.
type Engine struct {
	unit                *engine.Unit
	log                 zerolog.Logger
	backend             *backend.Backend // the gRPC service implementation
	unsecureGrpcServer  *grpc.Server     // the unsecure gRPC server
	secureGrpcServer    *grpc.Server     // the secure gRPC server
	httpServer          *http.Server
	restServer          *http.Server
	config              Config
	chain               flow.Chain
	unsecureGrpcAddress net.Addr
	secureGrpcAddress   net.Addr
	restAPIAddress      net.Addr
}

// New returns a new RPC engine.
func New(log zerolog.Logger,
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
	collectionGRPCPort uint,
	executionGRPCPort uint,
	retryEnabled bool,
	rpcMetricsEnabled bool,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the Access API e.g. Ping->100, GetTransaction->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the Access API e.g. Ping->50, GetTransaction->10
) *Engine {

	log = log.With().Str("engine", "rpc").Logger()

	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	// create a GRPC server to serve GRPC clients
	grpcOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	}

	var interceptors []grpc.UnaryServerInterceptor // ordered list of interceptors
	// if rpc metrics is enabled, first create the grpc metrics interceptor
	if rpcMetricsEnabled {
		interceptors = append(interceptors, grpc_prometheus.UnaryServerInterceptor)
	}

	if len(apiRatelimits) > 0 {
		// create a rate limit interceptor
		rateLimitInterceptor := NewRateLimiterInterceptor(log, apiRatelimits, apiBurstLimits).unaryServerInterceptor
		// append the rate limit interceptor to the list of interceptors
		interceptors = append(interceptors, rateLimitInterceptor)
	}

	// add the logging interceptor, ensure it is innermost wrapper
	interceptors = append(interceptors, loggingInterceptor(log)...)

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	grpcOpts = append(grpcOpts, chainedInterceptors)

	// create an unsecured grpc server
	unsecureGrpcServer := grpc.NewServer(grpcOpts...)

	// create a secure server server by using the secure grpc credentials that are passed in as part of config
	grpcOpts = append(grpcOpts, grpc.Creds(config.TransportCredentials))
	secureGrpcServer := grpc.NewServer(grpcOpts...)

	// wrap the unsecured server with an HTTP proxy server to serve HTTP clients
	httpServer := NewHTTPServer(unsecureGrpcServer, config.HTTPListenAddr)

	connectionFactory := &backend.ConnectionFactoryImpl{
		CollectionGRPCPort:        collectionGRPCPort,
		ExecutionGRPCPort:         executionGRPCPort,
		CollectionNodeGRPCTimeout: config.CollectionClientTimeout,
		ExecutionNodeGRPCTimeout:  config.ExecutionClientTimeout,
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
	)

	eng := &Engine{
		log:                log,
		unit:               engine.NewUnit(),
		backend:            backend,
		unsecureGrpcServer: unsecureGrpcServer,
		secureGrpcServer:   secureGrpcServer,
		httpServer:         httpServer,
		config:             config,
		chain:              chainID.Chain(),
	}

	accessproto.RegisterAccessAPIServer(
		eng.unsecureGrpcServer,
		access.NewHandler(backend, chainID.Chain()),
	)

	accessproto.RegisterAccessAPIServer(
		eng.secureGrpcServer,
		access.NewHandler(backend, chainID.Chain()),
	)

	if rpcMetricsEnabled {
		// Not interested in legacy metrics, so initialize here
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(unsecureGrpcServer)
		grpc_prometheus.Register(secureGrpcServer)
	}

	// Register legacy gRPC handlers for backwards compatibility, to be removed at a later date
	legacyaccessproto.RegisterAccessAPIServer(
		eng.unsecureGrpcServer,
		legacyaccess.NewHandler(backend, chainID.Chain()),
	)
	legacyaccessproto.RegisterAccessAPIServer(
		eng.secureGrpcServer,
		legacyaccess.NewHandler(backend, chainID.Chain()),
	)

	return eng
}

// Ready returns a ready channel that is closed once the engine has fully
// started. The RPC engine is ready when the gRPC server has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.serveUnsecureGRPC)
	e.unit.Launch(e.serveSecureGRPC)
	e.unit.Launch(e.serveGRPCWebProxy)
	if e.config.RESTListenAddr != "" {
		e.unit.Launch(e.serveREST)
	}
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(
		e.unsecureGrpcServer.GracefulStop,
		e.secureGrpcServer.GracefulStop,
		func() {
			err := e.httpServer.Shutdown(context.Background())
			if err != nil {
				e.log.Error().Err(err).Msg("error stopping http server")
			}
		},
		func() {
			if e.restServer != nil {
				err := e.restServer.Shutdown(context.Background())
				if err != nil {
					e.log.Error().Err(err).Msg("error stopping http REST server")
				}
			}
		})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

func (e *Engine) UnsecureGRPCAddress() net.Addr {
	return e.unsecureGrpcAddress
}

func (e *Engine) SecureGRPCAddress() net.Addr {
	return e.secureGrpcAddress
}

func (e *Engine) RestApiAddress() net.Addr {
	return e.restAPIAddress
}

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(event interface{}) error {
	switch entity := event.(type) {
	case *flow.Block:
		e.backend.NotifyFinalizedBlockHeight(entity.Header.Height)
		return nil
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// serveUnsecureGRPC starts the unsecure gRPC server
// When this function returns, the server is considered ready.
func (e *Engine) serveUnsecureGRPC() {

	e.log.Info().Str("grpc_address", e.config.UnsecureGRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.UnsecureGRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		return
	}

	// save the actual address on which we are listening (may be different from e.config.UnsecureGRPCListenAddr if not port
	// was specified)
	e.unsecureGrpcAddress = l.Addr()

	e.log.Debug().Str("unsecure_grpc_address", e.unsecureGrpcAddress.String()).Msg("listening on port")

	err = e.unsecureGrpcServer.Serve(l) // blocking call
	if err != nil {
		e.log.Fatal().Err(err).Msg("fatal error in unsecure grpc server")
	}
}

// serveSecureGRPC starts the secure gRPC server
// When this function returns, the server is considered ready.
func (e *Engine) serveSecureGRPC() {

	e.log.Info().Str("secure_grpc_address", e.config.SecureGRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.SecureGRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		return
	}

	e.secureGrpcAddress = l.Addr()

	e.log.Debug().Str("secure_grpc_address", e.secureGrpcAddress.String()).Msg("listening on port")

	err = e.secureGrpcServer.Serve(l) // blocking call
	if err != nil {
		e.log.Fatal().Err(err).Msg("fatal error in secure grpc server")
	}
}

// serveGRPCWebProxy starts the gRPC web proxy server
func (e *Engine) serveGRPCWebProxy() {
	log := e.log.With().Str("http_proxy_address", e.config.HTTPListenAddr).Logger()

	log.Info().Msg("starting http proxy server on address")

	err := e.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return
	}
	if err != nil {
		e.log.Err(err).Msg("failed to start the http proxy server")
	}
}

// serveREST starts the HTTP REST server
func (e *Engine) serveREST() {

	e.log.Info().Str("rest_api_address", e.config.RESTListenAddr).Msg("starting REST server on address")

	r, err := rest.NewServer(e.backend, e.config.RESTListenAddr, e.log, e.chain)
	if err != nil {
		e.log.Err(err).Msg("failed to initialize the REST server")
		return
	}
	e.restServer = r

	l, err := net.Listen("tcp", e.config.RESTListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the REST server")
		return
	}

	e.restAPIAddress = l.Addr()

	e.log.Debug().Str("rest_api_address", e.restAPIAddress.String()).Msg("listening on port")

	err = e.restServer.Serve(l) // blocking call
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return
		}
		e.log.Error().Err(err).Msg("fatal error in REST server")
	}
}
