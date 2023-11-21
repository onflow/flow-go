package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // required for gRPC compression
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine"
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate" // required for gRPC compression
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"  // required for gRPC compression
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	exeEng "github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/scripts"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

const DefaultMaxBlockRange = 300

// Config defines the configurable options for the gRPC server.
type Config struct {
	ListenAddr        string
	MaxMsgSize        uint // In bytes
	RpcMetricsEnabled bool // enable GRPC metrics reporting
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
	scriptsExecutor exeEng.ScriptExecutor,
	headers storage.Headers,
	state protocol.State,
	events storage.Events,
	exeResults storage.ExecutionResults,
	txResults storage.TransactionResults,
	commits storage.Commits,
	chainID flow.ChainID,
	signerIndicesDecoder hotstuff.BlockSignerDecoder,
	apiRatelimits map[string]int, // the api rate limit (max calls per second) for each of the gRPC API e.g. Ping->100, ExecuteScriptAtBlockID->300
	apiBurstLimits map[string]int, // the api burst limit (max calls at the same time) for each of the gRPC API e.g. Ping->50, ExecuteScriptAtBlockID->10
) *Engine {
	log = log.With().Str("engine", "rpc").Logger()
	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(int(config.MaxMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxMsgSize)),
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

	// create a chained unary interceptor
	chainedInterceptors := grpc.ChainUnaryInterceptor(interceptors...)
	serverOptions = append(serverOptions, chainedInterceptors)

	server := grpc.NewServer(serverOptions...)

	eng := &Engine{
		log:  log,
		unit: engine.NewUnit(),
		handler: &handler{
			engine:               scriptsExecutor,
			chain:                chainID,
			headers:              headers,
			state:                state,
			signerIndicesDecoder: signerIndicesDecoder,
			events:               events,
			exeResults:           exeResults,
			transactionResults:   txResults,
			commits:              commits,
			log:                  log,
			maxBlockRange:        DefaultMaxBlockRange,
		},
		server: server,
		config: config,
	}

	if config.RpcMetricsEnabled {
		grpc_prometheus.EnableHandlingTimeHistogram()
		grpc_prometheus.Register(server)
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
	engine               exeEng.ScriptExecutor
	chain                flow.ChainID
	headers              storage.Headers
	state                protocol.State
	signerIndicesDecoder hotstuff.BlockSignerDecoder
	events               storage.Events
	exeResults           storage.ExecutionResults
	transactionResults   storage.TransactionResults
	log                  zerolog.Logger
	commits              storage.Commits
	maxBlockRange        int
}

var _ execution.ExecutionAPIServer = (*handler)(nil)

// Ping responds to requests when the server is up.
func (h *handler) Ping(
	_ context.Context,
	_ *execution.PingRequest,
) (*execution.PingResponse, error) {
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

	// return a more user friendly error if block has not been executed
	if _, err = h.commits.ByBlockID(blockID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "block %s has not been executed by node or was pruned", blockID)
		}
		return nil, status.Errorf(codes.Internal, "state commitment for block ID %s could not be retrieved", blockID)
	}

	value, err := h.engine.ExecuteScriptAtBlockID(ctx, req.GetScript(), req.GetArguments(), blockID)
	if err != nil {
		// todo check the error code instead
		// return code 3 as this passes the litmus test in our context
		return nil, status.Errorf(codes.InvalidArgument, "failed to execute script: %v", err)
	}

	res := &execution.ExecuteScriptAtBlockIDResponse{
		Value: value,
	}

	return res, nil
}

func (h *handler) GetRegisterAtBlockID(
	ctx context.Context,
	req *execution.GetRegisterAtBlockIDRequest,
) (*execution.GetRegisterAtBlockIDResponse, error) {

	blockID, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, err
	}

	owner := req.GetRegisterOwner()
	key := req.GetRegisterKey()
	value, err := h.engine.GetRegisterAtBlockID(ctx, owner, key, blockID)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to collect register  (owner : %s, key: %s): %v", hex.EncodeToString(owner), string(key), err)
	}

	res := &execution.GetRegisterAtBlockIDResponse{
		Value: value,
	}

	return res, nil
}

func (h *handler) GetEventsForBlockIDs(
	_ context.Context,
	req *execution.GetEventsForBlockIDsRequest,
) (*execution.GetEventsForBlockIDsResponse, error) {

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

	if len(blockIDs) > h.maxBlockRange {
		return nil, status.Errorf(codes.InvalidArgument, "too many block IDs requested: %d > %d", len(blockIDs), h.maxBlockRange)
	}

	results := make([]*execution.GetEventsForBlockIDsResponse_Result, len(blockIDs))

	// collect all the events and create a EventsResponse_Result for each block
	for i, bID := range flowBlockIDs {
		// Check if block has been executed
		if _, err := h.commits.ByBlockID(bID); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "block %s has not been executed by node or was pruned", bID)
			}
			return nil, status.Errorf(codes.Internal, "state commitment for block ID %s could not be retrieved", bID)
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
		Results:              results,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}, nil
}

func (h *handler) GetTransactionResult(
	_ context.Context,
	req *execution.GetTransactionResultRequest,
) (*execution.GetTransactionResultResponse, error) {

	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	reqTxID := req.GetTransactionId()
	txID, err := convert.TransactionID(reqTxID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transactionID: %v", err)
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
		cadenceErrMessage := txResult.ErrorMessage
		if !utf8.ValidString(cadenceErrMessage) {
			h.log.Warn().
				Str("block_id", blockID.String()).
				Str("transaction_id", txID.String()).
				Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
				Msg("invalid character in Cadence error message")
			// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
			cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
		}

		statusCode = 1 // for now a statusCode of 1 indicates an error and 0 indicates no error
		errMsg = cadenceErrMessage
	}

	// lookup events by block id and transaction ID
	blockEvents, err := h.events.ByBlockIDTransactionID(blockID, txID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
	}

	events := convert.EventsToMessages(blockEvents)

	// compose a response with the events and the transaction error
	return &execution.GetTransactionResultResponse{
		StatusCode:           statusCode,
		ErrorMessage:         errMsg,
		Events:               events,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}, nil
}

func (h *handler) GetTransactionResultByIndex(
	_ context.Context,
	req *execution.GetTransactionByIndexRequest,
) (*execution.GetTransactionResultResponse, error) {

	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	index := req.GetIndex()

	var statusCode uint32 = 0
	errMsg := ""

	// lookup any transaction error that might have occurred
	txResult, err := h.transactionResults.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction result not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction result: %v", err)
	}

	if txResult.ErrorMessage != "" {
		cadenceErrMessage := txResult.ErrorMessage
		if !utf8.ValidString(cadenceErrMessage) {
			h.log.Warn().
				Str("block_id", blockID.String()).
				Uint32("index", index).
				Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
				Msg("invalid character in Cadence error message")
			// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
			cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
		}

		statusCode = 1 // for now a statusCode of 1 indicates an error and 0 indicates no error
		errMsg = cadenceErrMessage
	}

	// lookup events by block id and transaction index
	txEvents, err := h.events.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
	}

	events := convert.EventsToMessages(txEvents)

	// compose a response with the events and the transaction error
	return &execution.GetTransactionResultResponse{
		StatusCode:           statusCode,
		ErrorMessage:         errMsg,
		Events:               events,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}, nil
}

func (h *handler) GetTransactionResultsByBlockID(
	_ context.Context,
	req *execution.GetTransactionsByBlockIDRequest,
) (*execution.GetTransactionResultsResponse, error) {

	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	// must verify block was locally executed first since transactionResults.ByBlockID will return
	// an empty slice if block does not exist
	if _, err = h.commits.ByBlockID(blockID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "block %s has not been executed by node or was pruned", blockID)
		}
		return nil, status.Errorf(codes.Internal, "state commitment for block ID %s could not be retrieved", blockID)
	}

	// Get all tx results
	txResults, err := h.transactionResults.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction results not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction result: %v", err)
	}

	// get all events for a block
	blockEvents, err := h.events.ByBlockID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for block: %v", err)
	}

	responseTxResults := make([]*execution.GetTransactionResultResponse, len(txResults))

	eventsByTxIndex := make(map[uint32][]flow.Event, len(txResults)) //we will have at most as many buckets as tx results

	// re-partition events by tx index
	// it's not documented but events are stored indexed by (blockID, event.TransactionID, event.TransactionIndex, event.EventIndex)
	// hence they should keep order within a transaction, so we don't sort resulting events slices
	for _, event := range blockEvents {
		eventsByTxIndex[event.TransactionIndex] = append(eventsByTxIndex[event.TransactionIndex], event)
	}

	// match tx results with events
	for index, txResult := range txResults {
		var statusCode uint32 = 0
		errMsg := ""

		txIndex := uint32(index)

		if txResult.ErrorMessage != "" {
			cadenceErrMessage := txResult.ErrorMessage
			if !utf8.ValidString(cadenceErrMessage) {
				h.log.Warn().
					Str("block_id", blockID.String()).
					Uint32("index", txIndex).
					Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
					Msg("invalid character in Cadence error message")
				// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
				cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
			}

			statusCode = 1 // for now a statusCode of 1 indicates an error and 0 indicates no error
			errMsg = cadenceErrMessage
		}

		events := convert.EventsToMessages(eventsByTxIndex[txIndex])

		responseTxResults[index] = &execution.GetTransactionResultResponse{
			StatusCode:   statusCode,
			ErrorMessage: errMsg,
			Events:       events,
		}
	}

	// compose a response
	return &execution.GetTransactionResultsResponse{
		TransactionResults:   responseTxResults,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}, nil
}

// GetTransactionErrorMessage implements a grpc handler for getting a transaction error message by block ID and tx ID.
// WARNING: we don't fill tx index in response since it's not available without iterating over all tx results.
// Expected error codes during normal operations:
// - codes.InvalidArgument - invalid blockID, tx ID.
// - codes.NotFound - transaction result by tx ID not found.
func (h *handler) GetTransactionErrorMessage(
	_ context.Context,
	req *execution.GetTransactionErrorMessageRequest,
) (*execution.GetTransactionErrorMessageResponse, error) {
	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	reqTxID := req.GetTransactionId()
	txID, err := convert.TransactionID(reqTxID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transactionID: %v", err)
	}

	// lookup any transaction error that might have occurred
	txResult, err := h.transactionResults.ByBlockIDTransactionID(blockID, txID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction result not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction result: %v", err)
	}

	result := &execution.GetTransactionErrorMessageResponse{
		TransactionId: convert.IdentifierToMessage(txResult.TransactionID),
	}

	if len(txResult.ErrorMessage) > 0 {
		cadenceErrMessage := txResult.ErrorMessage
		if !utf8.ValidString(cadenceErrMessage) {
			h.log.Warn().
				Str("block_id", blockID.String()).
				Str("transaction_id", txID.String()).
				Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
				Msg("invalid character in Cadence error message")
			// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
			cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
		}
		result.ErrorMessage = cadenceErrMessage
	}
	return result, nil
}

// GetTransactionErrorMessageByIndex implements a grpc handler for getting a transaction error message by block ID and tx index.
// Expected error codes during normal operations:
// - codes.InvalidArgument - invalid blockID.
// - codes.NotFound - transaction result at index not found.
func (h *handler) GetTransactionErrorMessageByIndex(
	_ context.Context,
	req *execution.GetTransactionErrorMessageByIndexRequest,
) (*execution.GetTransactionErrorMessageResponse, error) {
	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	index := req.GetIndex()

	// lookup any transaction error that might have occurred
	txResult, err := h.transactionResults.ByBlockIDTransactionIndex(blockID, index)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction result not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction result: %v", err)
	}

	result := &execution.GetTransactionErrorMessageResponse{
		TransactionId: convert.IdentifierToMessage(txResult.TransactionID),
	}

	if len(txResult.ErrorMessage) > 0 {
		cadenceErrMessage := txResult.ErrorMessage
		if !utf8.ValidString(cadenceErrMessage) {
			h.log.Warn().
				Str("block_id", blockID.String()).
				Str("transaction_id", txResult.TransactionID.String()).
				Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
				Msg("invalid character in Cadence error message")
			// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
			cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
		}
		result.ErrorMessage = cadenceErrMessage
	}
	return result, nil
}

// GetTransactionErrorMessagesByBlockID implements a grpc handler for getting transaction error messages by block ID.
// Only failed transactions will be returned.
// Expected error codes during normal operations:
// - codes.InvalidArgument - invalid blockID.
// - codes.NotFound - block was not executed or was pruned.
func (h *handler) GetTransactionErrorMessagesByBlockID(
	_ context.Context,
	req *execution.GetTransactionErrorMessagesByBlockIDRequest,
) (*execution.GetTransactionErrorMessagesResponse, error) {
	reqBlockID := req.GetBlockId()
	blockID, err := convert.BlockID(reqBlockID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	// must verify block was locally executed first since transactionResults.ByBlockID will return
	// an empty slice if block does not exist
	if _, err = h.commits.ByBlockID(blockID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "block %s has not been executed by node or was pruned", blockID)
		}
		return nil, status.Errorf(codes.Internal, "state commitment for block ID %s could not be retrieved", blockID)
	}

	// Get all tx results
	txResults, err := h.transactionResults.ByBlockID(blockID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "transaction results not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get transaction results: %v", err)
	}

	var results []*execution.GetTransactionErrorMessagesResponse_Result
	for index, txResult := range txResults {
		if len(txResult.ErrorMessage) == 0 {
			continue
		}
		txIndex := uint32(index)
		cadenceErrMessage := txResult.ErrorMessage
		if !utf8.ValidString(cadenceErrMessage) {
			h.log.Warn().
				Str("block_id", blockID.String()).
				Uint32("index", txIndex).
				Str("error_mgs", fmt.Sprintf("%q", cadenceErrMessage)).
				Msg("invalid character in Cadence error message")
			// convert non UTF-8 string to a UTF-8 string for safe GRPC marshaling
			cadenceErrMessage = strings.ToValidUTF8(txResult.ErrorMessage, "?")
		}
		results = append(results, &execution.GetTransactionErrorMessagesResponse_Result{
			TransactionId: convert.IdentifierToMessage(txResult.TransactionID),
			Index:         txIndex,
			ErrorMessage:  cadenceErrMessage,
		})
	}

	return &execution.GetTransactionErrorMessagesResponse{
		Results: results,
	}, nil
}

// eventResult creates EventsResponse_Result from flow.Event for the given blockID
func (h *handler) eventResult(
	blockID flow.Identifier,
	flowEvents []flow.Event,
) (*execution.GetEventsForBlockIDsResponse_Result, error) {

	// convert events to event message
	events := convert.EventsToMessages(flowEvents)

	// lookup block
	header, err := h.headers.ByBlockID(blockID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to lookup block: %v", err)
	}

	return &execution.GetEventsForBlockIDsResponse_Result{
		BlockId:     blockID[:],
		BlockHeight: header.Height,
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
		return nil, status.Errorf(codes.InvalidArgument, "invalid blockID: %v", err)
	}

	flowAddress, err := convert.Address(req.GetAddress(), h.chain.Chain())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	// return a more user friendly error if block has not been executed
	if _, err = h.commits.ByBlockID(blockFlowID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "block %s has not been executed by node or was pruned", blockFlowID)
		}
		return nil, status.Errorf(codes.Internal, "state commitment for block ID %s could not be retrieved", blockFlowID)
	}

	value, err := h.engine.GetAccount(ctx, flowAddress, blockFlowID)
	if err != nil {
		if errors.Is(err, scripts.ErrStateCommitmentPruned) {
			return nil, status.Errorf(codes.OutOfRange, "state for block ID %s not available", blockFlowID)
		}
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "account with address %s not found", flowAddress)
		}
		if fvmerrors.IsAccountNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "account not found")
		}
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

// GetLatestBlockHeader gets the latest sealed or finalized block header.
func (h *handler) GetLatestBlockHeader(
	_ context.Context,
	req *execution.GetLatestBlockHeaderRequest,
) (*execution.BlockHeaderResponse, error) {
	var header *flow.Header
	var err error

	if req.GetIsSealed() {
		// get the latest seal header from storage
		header, err = h.state.Sealed().Head()
	} else {
		// get the finalized header from state
		header, err = h.state.Final().Head()
	}
	if err != nil {
		// this header MUST exist in the db, otherwise the node likely has inconsistent state.
		// Don't crash as a result of an external API request, but other components will likely panic.
		h.log.Err(err).Msg("failed to get latest block header. potentially inconsistent protocol state.")
		return nil, status.Errorf(codes.Internal, "unable to get latest header: %v", err)
	}

	return h.blockHeaderResponse(header)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *handler) GetBlockHeaderByID(
	_ context.Context,
	req *execution.GetBlockHeaderByIDRequest,
) (*execution.BlockHeaderResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, err
	}
	header, err := h.headers.ByBlockID(id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "not found: %v", err)
	}
	return h.blockHeaderResponse(header)
}

func (h *handler) blockHeaderResponse(header *flow.Header) (*execution.BlockHeaderResponse, error) {
	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(header)
	if err != nil {
		// the block was retrieved from local storage - so no errors are expected
		return nil, fmt.Errorf("failed to decode signer indices to Identifiers for block %v: %w", header.ID(), err)
	}

	msg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, err
	}

	return &execution.BlockHeaderResponse{
		Block: msg,
	}, nil
}
