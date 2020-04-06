package rpc

import (
	"context"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/model/flow"
	entities "github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	access "github.com/dapperlabs/flow-go/protobuf/services/access"
	"github.com/dapperlabs/flow-go/storage"
)

// Config defines the configurable options for the gRPC server.
type Config struct {
	ListenAddr string
}

// Engine implements a gRPC server with a simplified version of the Observation API.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	handler *handler     // the gRPC service implementation
	server  *grpc.Server // the gRPC server
	config  Config
}

// New returns a new RPC engine.
func New(log zerolog.Logger, config Config, e *ingestion.Engine, events storage.Events) *Engine {
	log = log.With().Str("engine", "rpc").Logger()

	eng := &Engine{
		log:  log,
		unit: engine.NewUnit(),
		handler: &handler{
			UnimplementedAccessAPIServer: access.UnimplementedAccessAPIServer{},
			engine:                       e,
			events:                       events,
		},
		server: grpc.NewServer(),
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

// handler implements a subset of the Observation API.
type handler struct {
	access.UnimplementedAccessAPIServer
	engine *ingestion.Engine
	events storage.Events
}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *access.PingRequest) (*access.PingResponse, error) {
	return &access.PingResponse{}, nil
}

func (h *handler) ExecuteScriptAtLatestBlock(_ context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {

	value, err := h.engine.ExecuteScript(req.Script)
	if err != nil {
		return nil, err
	}

	res := &access.ExecuteScriptResponse{
		Value: value,
	}

	return res, nil
}

func (h *handler) GetEventsForBlockIDs(_ context.Context,
	req *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {

	blockIDs := req.GetBlockIds()
	if len(blockIDs) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no block IDs provided")
	}

	eType := flow.EventType(req.Type)

	events := make([]*entities.Event, 0)

	// collect all the events in events
	for _, b := range blockIDs {
		bID := flow.HashToID(b)

		flowEvents, err := h.events.ByBlockIDEventType(bID, eType)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
		}

		for _, e := range flowEvents {
			event := convert.EventToMessage(e)
			events = append(events, event)
		}
	}

	return &access.EventsResponse{
		Events: events,
	}, nil
}
