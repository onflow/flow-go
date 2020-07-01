package rpc

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/access/rpc/handler"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	grpcutils "github.com/dapperlabs/flow-go/utils/grpc"
)

// Config defines the configurable options for the access node server
type Config struct {
	GRPCListenAddr string
	HTTPListenAddr string
	ExecutionAddr  string
	CollectionAddr string
	MaxMsgSize     int // In bytes
}

// Engine implements a gRPC server with a simplified version of the Observation API.
type Engine struct {
	unit       *engine.Unit
	log        zerolog.Logger
	handler    *handler.Handler // the gRPC service implementation
	grpcServer *grpc.Server     // the gRPC server
	httpServer *http.Server
	config     Config
}

// New returns a new RPC engine.
func New(log zerolog.Logger,
	state protocol.State,
	config Config,
	executionRPC execution.ExecutionAPIClient,
	collectionRPC access.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	chainID flow.ChainID) *Engine {

	log = log.With().Str("engine", "rpc").Logger()

	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	// create a GRPC server to serve GRPC clients
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(config.MaxMsgSize),
		grpc.MaxSendMsgSize(config.MaxMsgSize),
	)

	// wrap the GRPC server with an HTTP proxy server to serve HTTP clients
	httpServer := NewHTTPServer(grpcServer, config.HTTPListenAddr)

	eng := &Engine{
		log:        log,
		unit:       engine.NewUnit(),
		handler:    handler.NewHandler(log, state, executionRPC, collectionRPC, blocks, headers, collections, transactions, chainID),
		grpcServer: grpcServer,
		httpServer: httpServer,
		config:     config,
	}

	access.RegisterAccessAPIServer(eng.grpcServer, eng.handler)

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

// serveGRPC starts the gRPC server
// When this function returns, the server is considered ready.
func (e *Engine) serveGRPC() {
	e.log.Info().Msgf("starting grpc server on address %s", e.config.GRPCListenAddr)

	l, err := net.Listen("tcp", e.config.GRPCListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start the grpc server")
		return
	}

	err = e.grpcServer.Serve(l)
	if err != nil {
		e.log.Err(err).Msg("fatal error in grpc server")
	}
}

// serveGRPCWebProxy starts the gRPC web proxy server
func (e *Engine) serveGRPCWebProxy() {
	e.log.Info().Msgf("starting http proxy server on address %s", e.config.HTTPListenAddr)
	err := e.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return
	}
	if err != nil {
		e.log.Err(err).Msg("failed to start the http proxy server")
	}
}
