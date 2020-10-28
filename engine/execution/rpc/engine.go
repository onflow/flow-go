package rpc

import (
	"context"
	"errors"
	"net"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/engine/execution/ingestion"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	grpcutils "github.com/onflow/flow-go/utils/grpc"
)

// Config defines the configurable options for the gRPC server.
type Config struct {
	ListenAddr string
	MaxMsgSize int // In bytes
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
func New(
	log zerolog.Logger,
	config Config,
	e *ingestion.Engine,
	blocks storage.Blocks,
	events storage.Events,
	exeResults storage.ExecutionResults,
	txResults storage.TransactionResults,
	chainID flow.ChainID) *Engine {
	log = log.With().Str("engine", "rpc").Logger()

	if config.MaxMsgSize == 0 {
		config.MaxMsgSize = grpcutils.DefaultMaxMsgSize
	}

	eng := &Engine{
		log:  log,
		unit: engine.NewUnit(),
		handler: &handler{
			engine:             e,
			chain:              chainID,
			blocks:             blocks,
			events:             events,
			exeResults:         exeResults,
			transactionResults: txResults,
		},
		server: grpc.NewServer(
			grpc.MaxRecvMsgSize(config.MaxMsgSize),
			grpc.MaxSendMsgSize(config.MaxMsgSize),
		),
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
	engine             ingestion.IngestRPC
	chain              flow.ChainID
	blocks             storage.Blocks
	events             storage.Events
	exeResults         storage.ExecutionResults
	transactionResults storage.TransactionResults
}

var _ execution.ExecutionAPIServer = &handler{}

// Ping responds to requests when the server is up.
func (h *handler) Ping(ctx context.Context, req *execution.PingRequest) (*execution.PingResponse, error) {
	return &execution.PingResponse{}, nil
}

func (h *handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *execution.ExecuteScriptAtBlockIDRequest,
) (*execution.ExecuteScriptAtBlockIDResponse, error) {

	blockID, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, err
	}

	value, err := h.engine.ExecuteScriptAtBlockID(ctx, req.GetScript(), req.GetArguments(), blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute script: %v", err)
	}

	res := &execution.ExecuteScriptAtBlockIDResponse{
		Value: value,
	}

	return res, nil
}

func (h *handler) GetEventsForBlockIDs(_ context.Context,
	req *execution.GetEventsForBlockIDsRequest) (*execution.GetEventsForBlockIDsResponse, error) {

	// validate request
	blockIDs := req.GetBlockIds()
	flowBlockIDs, err := convert.BlockIDs(blockIDs)
	if err != nil {
		return nil, err
	}
	reqEvent := req.GetType()
	eType, err := convert.EventType(reqEvent)
	if err != nil {
		return nil, err
	}

	results := make([]*execution.GetEventsForBlockIDsResponse_Result, len(blockIDs))

	// collect all the events and create a EventsResponse_Result for each block
	for i, bID := range flowBlockIDs {
		// Check if block has been executed
		if _, err := h.exeResults.ByBlockID(bID); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "results for block ID %s does not exist", bID)
			}
			return nil, status.Errorf(codes.Internal, "results for block ID %s could not be retrieved", bID)
		}

		// lookup events
		blockEvents, err := h.events.ByBlockIDEventType(bID, flow.EventType(eType))
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
		}

		result, err := h.eventResult(bID, blockEvents)
		if err != nil {
			return nil, err
		}
		results[i] = result

	}

	return &execution.GetEventsForBlockIDsResponse{
		Results: results,
	}, nil
}

func (h *handler) GetTransactionResult(
	_ context.Context,
	req *execution.GetTransactionResultRequest,
) (*execution.GetTransactionResultResponse, error) {

	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, err
	}

	reqTxID := req.GetTransactionId()
	txID, err := convert.TransactionID(reqTxID)
	if err != nil {
		return nil, err
	}

	var statusCode uint32 = 0
	errMsg := ""

	// lookup any transaction error that might have occurred
	txResult, err := h.transactionResults.ByBlockIDTransactionID(blockID, txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction result not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction result: %v", err)
	}
	if txResult.ErrorMessage != "" {
		statusCode = 1 // for now a statusCode of 1 indicates an error and 0 indicates no error
		errMsg = txResult.ErrorMessage
	}

	// lookup events by block id and transaction ID
	blockEvents, err := h.events.ByBlockIDTransactionID(blockID, txID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
	}

	events := convert.EventsToMessages(blockEvents)

	// compose a response with the events and the transaction error
	return &execution.GetTransactionResultResponse{
		StatusCode:   statusCode,
		ErrorMessage: errMsg,
		Events:       events,
	}, nil
}

// eventResult creates EventsResponse_Result from flow.Event for the given blockID
func (h *handler) eventResult(blockID flow.Identifier,
	flowEvents []flow.Event) (*execution.GetEventsForBlockIDsResponse_Result, error) {

	// convert events to event message
	events := convert.EventsToMessages(flowEvents)

	// lookup block
	block, err := h.blocks.ByID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup block: %v", err)
	}

	return &execution.GetEventsForBlockIDsResponse_Result{
		BlockId:     blockID[:],
		BlockHeight: block.Header.Height,
		Events:      events,
	}, nil
}

func (h *handler) GetAccountAtBlockID(
	ctx context.Context,
	req *execution.GetAccountAtBlockIDRequest,
) (*execution.GetAccountAtBlockIDResponse, error) {

	blockID := req.GetBlockId()
	blockFlowID, err := convert.BlockID(blockID)
	if err != nil {
		return nil, err
	}

	flowAddress, err := convert.Address(req.GetAddress(), h.chain.Chain())
	if err != nil {
		return nil, err
	}

	value, err := h.engine.GetAccount(ctx, flowAddress, blockFlowID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get account: %v", err)
	}

	if value == nil {
		return nil, status.Errorf(codes.NotFound, "account with address %s does not exist", flowAddress)
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
