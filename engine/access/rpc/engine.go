package rpc

import (
	"fmt"
	"net"

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

// Config defines the configurable options for the gRPC server.
type Config struct {
	ListenAddr     string
	ExecutionAddr  string
	CollectionAddr string
	MaxMsgSize     int // In bytes
}

// Engine implements a gRPC server with a simplified version of the Observation API.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	handler *handler.Handler // the gRPC service implementation
	server  *grpc.Server     // the gRPC server
	config  Config
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
	transactions storage.Transactions) *Engine {

	log = log.With().Str("engine", "rpc").Logger()

	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	eng := &Engine{
		log:     log,
		unit:    engine.NewUnit(),
		handler: handler.NewHandler(log, state, executionRPC, collectionRPC, blocks, headers, collections, transactions),
		server: grpc.NewServer(
			grpc.MaxRecvMsgSize(config.MaxMsgSize),
			grpc.MaxSendMsgSize(config.MaxMsgSize),
		),
		config: config,
	}

	access.RegisterAccessAPIServer(eng.server, eng.handler)

	return eng
}

// Ready returns a ready channel that is closed once the engine has fully
// started. The RPC engine is ready when the gRPC server has successfully
// started.
func (e *Engine) Ready() <-chan struct{} {
	e.unit.Launch(e.serve)
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It sends a signal to stop the gRPC server, then closes the channel.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(e.server.GracefulStop)
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

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(event interface{}) error {
	switch entity := event.(type) {
	case *flow.Block:
		e.handler.NotifyFinalizedBlockHeight(entity.Header.Height)
		return nil
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// serve starts the gRPC server .
//
// When this function returns, the server is considered ready.
func (e *Engine) serve() {
	e.log.Info().Msgf("starting server on address %s", e.config.ListenAddr)

	l, err := net.Listen("tcp", e.config.ListenAddr)
	if err != nil {
		e.log.Err(err).Msg("failed to start server")
		return
	}

	err = e.server.Serve(l)
	if err != nil {
		e.log.Err(err).Msg("fatal error in server")
	}
}
