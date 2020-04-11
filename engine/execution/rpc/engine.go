package rpc

import (
	"context"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow/protobuf/go/flow/entities"
	"github.com/dapperlabs/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/engine/execution/ingestion"
	"github.com/dapperlabs/flow-go/model/flow"
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
func New(log zerolog.Logger, config Config, e *ingestion.Engine, blocks storage.Blocks, events storage.Events) *Engine {
	log = log.With().Str("engine", "rpc").Logger()

	eng := &Engine{
		log:  log,
		unit: engine.NewUnit(),
		handler: &handler{
			engine: e,
			blocks: blocks,
			events: events,
		},
		server: grpc.NewServer(),
		config: config,
	}

	execution.RegisterExecutionAPIServer(eng.server, eng.handler)

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
	engine ingestion.IngestRPC
	blocks storage.Blocks
	events storage.Events
}

var _ execution.ExecutionAPIServer = &handler{}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *execution.PingRequest) (*execution.PingResponse, error) {
	return &execution.PingResponse{}, nil
}

func (h *handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *execution.ExecuteScriptAtBlockIDRequest,
) (*execution.ExecuteScriptResponse, error) {
	blockID := flow.HashToID(req.GetBlockId())

	value, err := h.engine.ExecuteScriptAtBlockID(req.Script, blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute script: %v", err)
	}

	res := &execution.ExecuteScriptResponse{
		Value: value,
	}

	return res, nil
}

func (h *handler) GetEventsForBlockIDs(_ context.Context,
	req *execution.GetEventsForBlockIDsRequest) (*execution.EventsResponse, error) {

	blockIDs := req.GetBlockIds()
	if len(blockIDs) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no block IDs provided")
	}

	reqEvent := req.GetType()
	if reqEvent == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid event type")
	}

	eType := flow.EventType(reqEvent)

	results := make([]*execution.EventsResponse_Result, len(blockIDs))

	// collect all the events and create a EventsResponse_Result for each block
	for i, b := range blockIDs {
		bID := flow.HashToID(b)

		// lookup events
		blockEvents, err := h.events.ByBlockIDEventType(bID, eType)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
		}

		result, err := h.eventResult(bID, blockEvents)
		if err != nil {
			return nil, err
		}
		results[i] = result

	}

	return &execution.EventsResponse{
		Results: results,
	}, nil
}

func (h *handler) GetEventsForBlockIDTransactionID(_ context.Context,
	req *execution.GetEventsForBlockIDTransactionIDRequest) (*execution.EventsResponse, error) {

	reqBlockID := req.GetBlockId()
	if reqBlockID == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id")
	}

	reqTxID := req.GetTransactionId()
	if reqTxID == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id")
	}

	bID := flow.HashToID(reqBlockID)
	tID := flow.HashToID(reqTxID)

	// lookup events by block id and transaction ID
	blockEvents, err := h.events.ByBlockIDTransactionID(bID, tID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
	}

	result, err := h.eventResult(bID, blockEvents)
	if err != nil {
		return nil, err
	}

	results := []*execution.EventsResponse_Result{
		result,
	}

	return &execution.EventsResponse{
		Results: results,
	}, nil
}

// eventResult creates EventsResponse_Result from flow.Event for the given blockID
func (h *handler) eventResult(blockID flow.Identifier,
	flowEvents []flow.Event) (*execution.EventsResponse_Result, error) {

	// convert events to event message
	events := make([]*entities.Event, len(flowEvents))
	for i, e := range flowEvents {
		event := convert.EventToMessage(e)
		events[i] = event
	}

	// lookup block
	block, err := h.blocks.ByID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup block: %v", err)
	}

	return &execution.EventsResponse_Result{
		BlockId:     blockID[:],
		BlockHeight: block.Height,
		Events:      events,
	}, nil
}

func (h *handler) GetAccountAtBlockID(_ context.Context,
	req *execution.GetAccountAtBlockIDRequest) (*execution.GetAccountAtBlockIDResponse, error) {

	blockID := req.GetBlockId()
	if blockID == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block ID")
	}
	blockFlowID := flow.HashToID(blockID)

	address := req.GetAddress()
	if address == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address")
	}
	flowAddress := flow.BytesToAddress(address)

	value, err := h.engine.GetAccount(flowAddress, blockFlowID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get account: %v", err)
	}

	account, err := convert.AccountToMessage(value)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert account to message: %v", err)
	}

	res := &execution.GetAccountAtBlockIDResponse{
		Account: account,
	}

	return res, nil

}
