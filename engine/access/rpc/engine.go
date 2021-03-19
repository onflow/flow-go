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
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	legacyaccessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/access"
	legacyaccess "github.com/onflow/flow-go/access/legacy"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

// Config defines the configurable options for the access node server
type Config struct {
	GRPCListenAddr          string        // the GRPC server address as ip:port
	HTTPListenAddr          string        // the HTTP web proxy address as ip:port
	ExecutionAddr           string        // the address of the upstream execution node
	CollectionAddr          string        // the address of the upstream collection node
	HistoricalAccessAddrs   string        // the list of all access nodes from previous spork
	MaxMsgSize              int           // GRPC max message size
	ExecutionClientTimeout  time.Duration // execution API GRPC client timeout
	CollectionClientTimeout time.Duration // collection API GRPC client timeout
}

// Engine implements a gRPC server with a simplified version of the Observation API.
type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	backend     *backend.Backend // the gRPC service implementation
	grpcServer  *grpc.Server     // the gRPC server
	httpServer  *http.Server
	config      Config
	grpcAddress net.Addr
}

// New returns a new RPC engine.
func New(log zerolog.Logger,
	state protocol.State,
	config Config,
	executionRPC execproto.ExecutionAPIClient,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
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

	// add the logging interceptor
	interceptors = append(interceptors, loggingInterceptor(log)...)

	if len(apiRatelimits) > 0 {
		// create a rate limit interceptor
		rateLimitInterceptor := NewRateLimiterInterceptor(log, apiRatelimits, apiBurstLimits).unaryServerInterceptor
		// append the rate limit interceptor to the list of interceptors
		interceptors = append(interceptors, rateLimitInterceptor)
	}

	if len(interceptors) > 0 {
		// create a chained unary interceptor
		chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
		grpcOpts = append(grpcOpts, chainedInterceptors)
	}

	grpcServer := grpc.NewServer(grpcOpts...)

	// wrap the GRPC server with an HTTP proxy server to serve HTTP clients
	httpServer := NewHTTPServer(grpcServer, config.HTTPListenAddr)

	connectionFactory := &backend.ConnectionFactoryImpl{
		CollectionGRPCPort:        collectionGRPCPort,
		ExecutionGRPCPort:         executionGRPCPort,
		CollectionNodeGRPCTimeout: config.CollectionClientTimeout,
		ExecutionNodeGRPCTimeout:  config.ExecutionClientTimeout,
	}

	backend := backend.New(
		state,
		executionRPC,
		collectionRPC,
		historicalAccessNodes,
		blocks,
		headers,
		collections,
		transactions,
		executionReceipts,
		chainID,
		transactionMetrics,
		connectionFactory,
		retryEnabled,
		log,
	)

	eng := &Engine{
		log:        log,
		unit:       engine.NewUnit(),
		backend:    backend,
		grpcServer: grpcServer,
		httpServer: httpServer,
		config:     config,
	}

	accessproto.RegisterAccessAPIServer(
		eng.grpcServer,
		access.NewHandler(backend, chainID.Chain()),
	)

	if rpcMetricsEnabled {
		// Not interested in legacy metrics, so initialize here
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(grpcServer)
	}

	// Register legacy gRPC handlers for backwards compatibility, to be removed at a later date
	legacyaccessproto.RegisterAccessAPIServer(
		eng.grpcServer,
		legacyaccess.NewHandler(backend, chainID.Chain()),
	)

	return eng
}

// Ready returns a ready channel that is closed once the engine has fully
// started. The RPC engine is ready when the gRPC server has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.serveGRPC)
	e.unit.Launch(e.serveGRPCWebProxy)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(
		e.grpcServer.GracefulStop,
		func() {
			err := e.httpServer.Shutdown(context.Background())
			if err != nil {
				e.log.Error().Err(err).Msg("error stopping http server")
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

func (e *Engine) GRPCAddress() net.Addr {
	return e.grpcAddress
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

// serveGRPC starts the gRPC server
// When this function returns, the server is considered ready.
func (e *Engine) serveGRPC() {

	e.log.Info().Str("grpc_address", e.config.GRPCListenAddr).Msg("starting grpc server on address")

	l, err := net.Listen("tcp", e.config.GRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		return
	}

	// save the actual address on which we are listening (may be different from e.config.GRPCListenAddr if not port
	// was specified)
	e.grpcAddress = l.Addr()

	e.log.Debug().Str("grpc_address", e.grpcAddress.String()).Msg("listening on port")

	err = e.grpcServer.Serve(l) // blocking call
	if err != nil {
		e.log.Fatal().Err(err).Msg("fatal error in grpc server")
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
