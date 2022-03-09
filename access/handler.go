package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

type Handler struct {
	api   API
	chain flow.Chain
}

func NewHandler(api API, chain flow.Chain) *Handler {
	return &Handler{
		api:   api,
		chain: chain,
	}
}

// Ping the Access API server for a response.
func (h *Handler) Ping(ctx context.Context, _ *access.PingRequest) (*access.PingResponse, error) {
	err := h.api.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &access.PingResponse{}, nil
}

func (h *Handler) GetNetworkParameters(
	ctx context.Context,
	_ *access.GetNetworkParametersRequest,
) (*access.GetNetworkParametersResponse, error) {
	params := h.api.GetNetworkParameters(ctx)

	return &access.GetNetworkParametersResponse{
		ChainId: string(params.ChainID),
	}, nil
}

// GetLatestBlockHeader gets the latest sealed block header.
func (h *Handler) GetLatestBlockHeader(
	ctx context.Context,
	req *access.GetLatestBlockHeaderRequest,
) (*access.BlockHeaderResponse, error) {
	header, err := h.api.GetLatestBlockHeader(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}

	return blockHeaderResponse(header)
}

// GetBlockHeaderByHeight gets a block header by height.
func (h *Handler) GetBlockHeaderByHeight(
	ctx context.Context,
	req *access.GetBlockHeaderByHeightRequest,
) (*access.BlockHeaderResponse, error) {
	header, err := h.api.GetBlockHeaderByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}

	return blockHeaderResponse(header)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *Handler) GetBlockHeaderByID(
	ctx context.Context,
	req *access.GetBlockHeaderByIDRequest,
) (*access.BlockHeaderResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, err
	}

	header, err := h.api.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return blockHeaderResponse(header)
}

// GetLatestBlock gets the latest sealed block.
func (h *Handler) GetLatestBlock(
	ctx context.Context,
	req *access.GetLatestBlockRequest,
) (*access.BlockResponse, error) {
	block, err := h.api.GetLatestBlock(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetBlockByHeight gets a block by height.
func (h *Handler) GetBlockByHeight(
	ctx context.Context,
	req *access.GetBlockByHeightRequest,
) (*access.BlockResponse, error) {
	block, err := h.api.GetBlockByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetBlockByHeight gets a block by ID.
func (h *Handler) GetBlockByID(
	ctx context.Context,
	req *access.GetBlockByIDRequest,
) (*access.BlockResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, err
	}

	block, err := h.api.GetBlockByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return blockResponse(block)
}

// GetCollectionByID gets a collection by ID.
func (h *Handler) GetCollectionByID(
	ctx context.Context,
	req *access.GetCollectionByIDRequest,
) (*access.CollectionResponse, error) {
	id, err := convert.CollectionID(req.GetId())
	if err != nil {
		return nil, err
	}

	col, err := h.api.GetCollectionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	colMsg, err := convert.LightCollectionToMessage(col)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.CollectionResponse{
		Collection: colMsg,
	}, nil
}

// SendTransaction submits a transaction to the network.
func (h *Handler) SendTransaction(
	ctx context.Context,
	req *access.SendTransactionRequest,
) (*access.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx, err := convert.MessageToTransaction(txMsg, h.chain)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = h.api.SendTransaction(ctx, &tx)
	if err != nil {
		return nil, err
	}

	txID := tx.ID()

	return &access.SendTransactionResponse{
		Id: txID[:],
	}, nil
}

// GetTransaction gets a transaction by ID.
func (h *Handler) GetTransaction(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResponse, error) {
	id, err := convert.TransactionID(req.GetId())
	if err != nil {
		return nil, err
	}

	tx, err := h.api.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return &access.TransactionResponse{
		Transaction: convert.TransactionToMessage(*tx),
	}, nil
}

// GetTransactionResult gets a transaction by ID.
func (h *Handler) GetTransactionResult(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResultResponse, error) {
	id, err := convert.TransactionID(req.GetId())
	if err != nil {
		return nil, err
	}

	result, err := h.api.GetTransactionResult(ctx, id)
	if err != nil {
		return nil, err
	}

	return TransactionResultToMessage(result), nil
}

// GetAccount returns an account by address at the latest sealed block.
func (h *Handler) GetAccount(
	ctx context.Context,
	req *access.GetAccountRequest,
) (*access.GetAccountResponse, error) {
	address := flow.BytesToAddress(req.GetAddress())

	account, err := h.api.GetAccount(ctx, address)
	if err != nil {
		return nil, err
	}

	accountMsg, err := convert.AccountToMessage(account)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.GetAccountResponse{
		Account: accountMsg,
	}, nil
}

// GetAccountAtLatestBlock returns an account by address at the latest sealed block.
func (h *Handler) GetAccountAtLatestBlock(
	ctx context.Context,
	req *access.GetAccountAtLatestBlockRequest,
) (*access.AccountResponse, error) {
	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, err
	}

	account, err := h.api.GetAccountAtLatestBlock(ctx, address)
	if err != nil {
		return nil, err
	}

	accountMsg, err := convert.AccountToMessage(account)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.AccountResponse{
		Account: accountMsg,
	}, nil
}

func (h *Handler) GetAccountAtBlockHeight(
	ctx context.Context,
	req *access.GetAccountAtBlockHeightRequest,
) (*access.AccountResponse, error) {
	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, err
	}

	account, err := h.api.GetAccountAtBlockHeight(ctx, address, req.GetBlockHeight())
	if err != nil {
		return nil, err
	}

	accountMsg, err := convert.AccountToMessage(account)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.AccountResponse{
		Account: accountMsg,
	}, nil
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block.
func (h *Handler) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()

	value, err := h.api.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height.
func (h *Handler) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockHeightRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()
	blockHeight := req.GetBlockHeight()

	value, err := h.api.ExecuteScriptAtBlockHeight(ctx, blockHeight, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// ExecuteScriptAtBlockID executes a script at a specific block ID.
func (h *Handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockIDRequest,
) (*access.ExecuteScriptResponse, error) {
	script := req.GetScript()
	arguments := req.GetArguments()
	blockID := convert.MessageToIdentifier(req.GetBlockId())

	value, err := h.api.ExecuteScriptAtBlockID(ctx, blockID, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value: value,
	}, nil
}

// GetEventsForHeightRange returns events matching a query.
func (h *Handler) GetEventsForHeightRange(
	ctx context.Context,
	req *access.GetEventsForHeightRangeRequest,
) (*access.EventsResponse, error) {
	eventType, err := convert.EventType(req.GetType())
	if err != nil {
		return nil, err
	}

	startHeight := req.GetStartHeight()
	endHeight := req.GetEndHeight()

	results, err := h.api.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight)
	if err != nil {
		return nil, err
	}

	resultEvents, err := blockEventsToMessages(results)
	if err != nil {
		return nil, err
	}
	return &access.EventsResponse{
		Results: resultEvents,
	}, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (h *Handler) GetEventsForBlockIDs(
	ctx context.Context,
	req *access.GetEventsForBlockIDsRequest,
) (*access.EventsResponse, error) {
	eventType, err := convert.EventType(req.GetType())
	if err != nil {
		return nil, err
	}

	blockIDs, err := convert.BlockIDs(req.GetBlockIds())
	if err != nil {
		return nil, err
	}

	results, err := h.api.GetEventsForBlockIDs(ctx, eventType, blockIDs)
	if err != nil {
		return nil, err
	}

	resultEvents, err := blockEventsToMessages(results)
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results: resultEvents,
	}, nil
}

// GetLatestProtocolStateSnapshot returns the latest serializable Snapshot
func (h *Handler) GetLatestProtocolStateSnapshot(ctx context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	snapshot, err := h.api.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	return &access.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
	}, nil
}

// GetExecutionResultForBlockID returns the latest received execution result for the given block ID.
// AN might receive multiple receipts with conflicting results for unsealed blocks.
// If this case happens, since AN is not able to determine which result is the correct one until the block is sealed, it has to pick one result to respond to this query. For now, we return the result from the latest received receipt.
func (h *Handler) GetExecutionResultForBlockID(ctx context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	blockID := convert.MessageToIdentifier(req.GetBlockId())

	result, err := h.api.GetExecutionResultForBlockID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return executionResultToMessages(result)
}

func blockResponse(block *flow.Block) (*access.BlockResponse, error) {
	msg, err := convert.BlockToMessage(block)
	if err != nil {
		return nil, err
	}

	return &access.BlockResponse{
		Block: msg,
	}, nil
}

func blockHeaderResponse(header *flow.Header) (*access.BlockHeaderResponse, error) {
	msg, err := convert.BlockHeaderToMessage(header)
	if err != nil {
		return nil, err
	}

	return &access.BlockHeaderResponse{
		Block: msg,
	}, nil
}

func executionResultToMessages(er *flow.ExecutionResult) (*access.ExecutionResultForBlockIDResponse, error) {
	execResult, err := convert.ExecutionResultToMessage(er)
	if err != nil {
		return nil, err
	}
	return &access.ExecutionResultForBlockIDResponse{ExecutionResult: execResult}, nil
}

func blockEventsToMessages(blocks []flow.BlockEvents) ([]*access.EventsResponse_Result, error) {
	results := make([]*access.EventsResponse_Result, len(blocks))

	for i, block := range blocks {
		event, err := blockEventsToMessage(block)
		if err != nil {
			return nil, err
		}
		results[i] = event
	}

	return results, nil
}

func blockEventsToMessage(block flow.BlockEvents) (*access.EventsResponse_Result, error) {
	eventMessages := make([]*entities.Event, len(block.Events))
	for i, event := range block.Events {
		eventMessages[i] = convert.EventToMessage(event)
	}
	timestamp := timestamppb.New(block.BlockTimestamp)
	return &access.EventsResponse_Result{
		BlockId:        block.BlockID[:],
		BlockHeight:    block.BlockHeight,
		BlockTimestamp: timestamp,
		Events:         eventMessages,
	}, nil
}
