package access

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
)

type Handler struct {
	api                  API
	chain                flow.Chain
	signerIndicesDecoder hotstuff.BlockSignerDecoder
	finalizedHeaderCache module.FinalizedHeaderCache
	me                   module.Local
}

// TODO: this is implemented in https://github.com/onflow/flow-go/pull/4957, remove when merged
func (h *Handler) GetProtocolStateSnapshotByBlockID(ctx context.Context, request *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// TODO: this is implemented in https://github.com/onflow/flow-go/pull/4957, remove when merged
func (h *Handler) GetProtocolStateSnapshotByHeight(ctx context.Context, request *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// TODO: this is implemented in https://github.com/onflow/flow-go/pull/5049, remove when merged
func (h *Handler) GetSystemTransaction(context.Context, *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// TODO: this is implemented in https://github.com/onflow/flow-go/pull/5049, remove when merged
func (h *Handler) GetSystemTransactionResult(context.Context, *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// HandlerOption is used to hand over optional constructor parameters
type HandlerOption func(*Handler)

var _ access.AccessAPIServer = (*Handler)(nil)

func NewHandler(api API, chain flow.Chain, finalizedHeader module.FinalizedHeaderCache, me module.Local, options ...HandlerOption) *Handler {
	h := &Handler{
		api:                  api,
		chain:                chain,
		finalizedHeaderCache: finalizedHeader,
		me:                   me,
		signerIndicesDecoder: &signature.NoopBlockSignerDecoder{},
	}
	for _, opt := range options {
		opt(h)
	}
	return h
}

// Ping the Access API server for a response.
func (h *Handler) Ping(ctx context.Context, _ *access.PingRequest) (*access.PingResponse, error) {
	err := h.api.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return &access.PingResponse{}, nil
}

// GetNodeVersionInfo gets node version information such as semver, commit, sporkID, protocolVersion, etc
func (h *Handler) GetNodeVersionInfo(
	ctx context.Context,
	_ *access.GetNodeVersionInfoRequest,
) (*access.GetNodeVersionInfoResponse, error) {
	nodeVersionInfo, err := h.api.GetNodeVersionInfo(ctx)
	if err != nil {
		return nil, err
	}

	return &access.GetNodeVersionInfoResponse{
		Info: &entities.NodeVersionInfo{
			Semver:               nodeVersionInfo.Semver,
			Commit:               nodeVersionInfo.Commit,
			SporkId:              nodeVersionInfo.SporkId[:],
			ProtocolVersion:      nodeVersionInfo.ProtocolVersion,
			SporkRootBlockHeight: nodeVersionInfo.SporkRootBlockHeight,
			NodeRootBlockHeight:  nodeVersionInfo.NodeRootBlockHeight,
		},
	}, nil
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
	header, status, err := h.api.GetLatestBlockHeader(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header, status)
}

// GetBlockHeaderByHeight gets a block header by height.
func (h *Handler) GetBlockHeaderByHeight(
	ctx context.Context,
	req *access.GetBlockHeaderByHeightRequest,
) (*access.BlockHeaderResponse, error) {
	header, status, err := h.api.GetBlockHeaderByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header, status)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *Handler) GetBlockHeaderByID(
	ctx context.Context,
	req *access.GetBlockHeaderByIDRequest,
) (*access.BlockHeaderResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}
	header, status, err := h.api.GetBlockHeaderByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header, status)
}

// GetLatestBlock gets the latest sealed block.
func (h *Handler) GetLatestBlock(
	ctx context.Context,
	req *access.GetLatestBlockRequest,
) (*access.BlockResponse, error) {
	block, status, err := h.api.GetLatestBlock(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse(), status)
}

// GetBlockByHeight gets a block by height.
func (h *Handler) GetBlockByHeight(
	ctx context.Context,
	req *access.GetBlockByHeightRequest,
) (*access.BlockResponse, error) {
	block, status, err := h.api.GetBlockByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse(), status)
}

// GetBlockByID gets a block by ID.
func (h *Handler) GetBlockByID(
	ctx context.Context,
	req *access.GetBlockByIDRequest,
) (*access.BlockResponse, error) {
	id, err := convert.BlockID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}
	block, status, err := h.api.GetBlockByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse(), status)
}

// GetCollectionByID gets a collection by ID.
func (h *Handler) GetCollectionByID(
	ctx context.Context,
	req *access.GetCollectionByIDRequest,
) (*access.CollectionResponse, error) {
	metadata := h.buildMetadataResponse()

	id, err := convert.CollectionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
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
		Metadata:   metadata,
	}, nil
}

// SendTransaction submits a transaction to the network.
func (h *Handler) SendTransaction(
	ctx context.Context,
	req *access.SendTransactionRequest,
) (*access.SendTransactionResponse, error) {
	metadata := h.buildMetadataResponse()

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
		Id:       txID[:],
		Metadata: metadata,
	}, nil
}

// GetTransaction gets a transaction by ID.
func (h *Handler) GetTransaction(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResponse, error) {
	metadata := h.buildMetadataResponse()

	id, err := convert.TransactionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id: %v", err)
	}

	tx, err := h.api.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return &access.TransactionResponse{
		Transaction: convert.TransactionToMessage(*tx),
		Metadata:    metadata,
	}, nil
}

// GetTransactionResult gets a transaction by ID.
func (h *Handler) GetTransactionResult(
	ctx context.Context,
	req *access.GetTransactionRequest,
) (*access.TransactionResultResponse, error) {
	metadata := h.buildMetadataResponse()

	transactionID, err := convert.TransactionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id: %v", err)
	}

	blockId := flow.ZeroID
	requestBlockId := req.GetBlockId()
	if requestBlockId != nil {
		blockId, err = convert.BlockID(requestBlockId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
		}
	}

	collectionId := flow.ZeroID
	requestCollectionId := req.GetCollectionId()
	if requestCollectionId != nil {
		collectionId, err = convert.CollectionID(requestCollectionId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
		}
	}

	eventEncodingVersion := req.GetEventEncodingVersion()
	result, err := h.api.GetTransactionResult(ctx, transactionID, blockId, collectionId, eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	message := TransactionResultToMessage(result)
	message.Metadata = metadata

	return message, nil
}

func (h *Handler) GetTransactionResultsByBlockID(
	ctx context.Context,
	req *access.GetTransactionsByBlockIDRequest,
) (*access.TransactionResultsResponse, error) {
	metadata := h.buildMetadataResponse()

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	eventEncodingVersion := req.GetEventEncodingVersion()

	results, err := h.api.GetTransactionResultsByBlockID(ctx, id, eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	message := TransactionResultsToMessage(results)
	message.Metadata = metadata

	return message, nil
}

func (h *Handler) GetTransactionsByBlockID(
	ctx context.Context,
	req *access.GetTransactionsByBlockIDRequest,
) (*access.TransactionsResponse, error) {
	metadata := h.buildMetadataResponse()

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	transactions, err := h.api.GetTransactionsByBlockID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &access.TransactionsResponse{
		Transactions: convert.TransactionsToMessages(transactions),
		Metadata:     metadata,
	}, nil
}

// GetTransactionResultByIndex gets a transaction at a specific index for in a block that is executed,
// pending or finalized transactions return errors
func (h *Handler) GetTransactionResultByIndex(
	ctx context.Context,
	req *access.GetTransactionByIndexRequest,
) (*access.TransactionResultResponse, error) {
	metadata := h.buildMetadataResponse()

	blockID, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	eventEncodingVersion := req.GetEventEncodingVersion()

	result, err := h.api.GetTransactionResultByIndex(ctx, blockID, req.GetIndex(), eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	message := TransactionResultToMessage(result)
	message.Metadata = metadata

	return message, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (h *Handler) GetAccount(
	ctx context.Context,
	req *access.GetAccountRequest,
) (*access.GetAccountResponse, error) {
	metadata := h.buildMetadataResponse()

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	account, err := h.api.GetAccount(ctx, address)
	if err != nil {
		return nil, err
	}

	accountMsg, err := convert.AccountToMessage(account)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &access.GetAccountResponse{
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

// GetAccountAtLatestBlock returns an account by address at the latest sealed block.
func (h *Handler) GetAccountAtLatestBlock(
	ctx context.Context,
	req *access.GetAccountAtLatestBlockRequest,
) (*access.AccountResponse, error) {
	metadata := h.buildMetadataResponse()

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
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
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

func (h *Handler) GetAccountAtBlockHeight(
	ctx context.Context,
	req *access.GetAccountAtBlockHeightRequest,
) (*access.AccountResponse, error) {
	metadata := h.buildMetadataResponse()

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
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
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block.
func (h *Handler) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	req *access.ExecuteScriptAtLatestBlockRequest,
) (*access.ExecuteScriptResponse, error) {
	metadata := h.buildMetadataResponse()

	script := req.GetScript()
	arguments := req.GetArguments()

	value, err := h.api.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height.
func (h *Handler) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockHeightRequest,
) (*access.ExecuteScriptResponse, error) {
	metadata := h.buildMetadataResponse()

	script := req.GetScript()
	arguments := req.GetArguments()
	blockHeight := req.GetBlockHeight()

	value, err := h.api.ExecuteScriptAtBlockHeight(ctx, blockHeight, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// ExecuteScriptAtBlockID executes a script at a specific block ID.
func (h *Handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *access.ExecuteScriptAtBlockIDRequest,
) (*access.ExecuteScriptResponse, error) {
	metadata := h.buildMetadataResponse()

	script := req.GetScript()
	arguments := req.GetArguments()
	blockID := convert.MessageToIdentifier(req.GetBlockId())

	value, err := h.api.ExecuteScriptAtBlockID(ctx, blockID, script, arguments)
	if err != nil {
		return nil, err
	}

	return &access.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// GetEventsForHeightRange returns events matching a query.
func (h *Handler) GetEventsForHeightRange(
	ctx context.Context,
	req *access.GetEventsForHeightRangeRequest,
) (*access.EventsResponse, error) {
	metadata := h.buildMetadataResponse()

	eventType, err := convert.EventType(req.GetType())
	if err != nil {
		return nil, err
	}

	startHeight := req.GetStartHeight()
	endHeight := req.GetEndHeight()

	eventEncodingVersion := req.GetEventEncodingVersion()

	results, err := h.api.GetEventsForHeightRange(ctx, eventType, startHeight, endHeight, eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	resultEvents, err := convert.BlockEventsToMessages(results)
	if err != nil {
		return nil, err
	}
	return &access.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (h *Handler) GetEventsForBlockIDs(
	ctx context.Context,
	req *access.GetEventsForBlockIDsRequest,
) (*access.EventsResponse, error) {
	metadata := h.buildMetadataResponse()

	eventType, err := convert.EventType(req.GetType())
	if err != nil {
		return nil, err
	}

	blockIDs, err := convert.BlockIDs(req.GetBlockIds())
	if err != nil {
		return nil, err
	}

	eventEncodingVersion := req.GetEventEncodingVersion()

	results, err := h.api.GetEventsForBlockIDs(ctx, eventType, blockIDs, eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	resultEvents, err := convert.BlockEventsToMessages(results)
	if err != nil {
		return nil, err
	}

	return &access.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

// GetLatestProtocolStateSnapshot returns the latest serializable Snapshot
func (h *Handler) GetLatestProtocolStateSnapshot(ctx context.Context, req *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	metadata := h.buildMetadataResponse()

	snapshot, err := h.api.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	return &access.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
		Metadata:           metadata,
	}, nil
}

// GetExecutionResultForBlockID returns the latest received execution result for the given block ID.
// AN might receive multiple receipts with conflicting results for unsealed blocks.
// If this case happens, since AN is not able to determine which result is the correct one until the block is sealed, it has to pick one result to respond to this query. For now, we return the result from the latest received receipt.
func (h *Handler) GetExecutionResultForBlockID(ctx context.Context, req *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	metadata := h.buildMetadataResponse()

	blockID := convert.MessageToIdentifier(req.GetBlockId())

	result, err := h.api.GetExecutionResultForBlockID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return executionResultToMessages(result, metadata)
}

// GetExecutionResultByID returns the execution result for the given ID.
func (h *Handler) GetExecutionResultByID(ctx context.Context, req *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	metadata := h.buildMetadataResponse()

	blockID := convert.MessageToIdentifier(req.GetId())

	result, err := h.api.GetExecutionResultByID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	execResult, err := convert.ExecutionResultToMessage(result)
	if err != nil {
		return nil, err
	}
	return &access.ExecutionResultByIDResponse{
		ExecutionResult: execResult,
		Metadata:        metadata,
	}, nil
}

func (h *Handler) blockResponse(block *flow.Block, fullResponse bool, status flow.BlockStatus) (*access.BlockResponse, error) {
	metadata := h.buildMetadataResponse()

	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(block.Header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	var msg *entities.Block
	if fullResponse {
		msg, err = convert.BlockToMessage(block, signerIDs)
		if err != nil {
			return nil, err
		}
	} else {
		msg = convert.BlockToMessageLight(block)
	}

	return &access.BlockResponse{
		Block:       msg,
		BlockStatus: entities.BlockStatus(status),
		Metadata:    metadata,
	}, nil
}

func (h *Handler) blockHeaderResponse(header *flow.Header, status flow.BlockStatus) (*access.BlockHeaderResponse, error) {
	metadata := h.buildMetadataResponse()

	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	msg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, err
	}

	return &access.BlockHeaderResponse{
		Block:       msg,
		BlockStatus: entities.BlockStatus(status),
		Metadata:    metadata,
	}, nil
}

// buildMetadataResponse builds and returns the metadata response object.
func (h *Handler) buildMetadataResponse() *entities.Metadata {
	lastFinalizedHeader := h.finalizedHeaderCache.Get()
	blockId := lastFinalizedHeader.ID()
	nodeId := h.me.NodeID()

	return &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
	}
}

func executionResultToMessages(er *flow.ExecutionResult, metadata *entities.Metadata) (*access.ExecutionResultForBlockIDResponse, error) {
	execResult, err := convert.ExecutionResultToMessage(er)
	if err != nil {
		return nil, err
	}
	return &access.ExecutionResultForBlockIDResponse{
		ExecutionResult: execResult,
		Metadata:        metadata,
	}, nil
}

// WithBlockSignerDecoder configures the Handler to decode signer indices
// via the provided hotstuff.BlockSignerDecoder
func WithBlockSignerDecoder(signerIndicesDecoder hotstuff.BlockSignerDecoder) func(*Handler) {
	return func(handler *Handler) {
		handler.signerIndicesDecoder = signerIndicesDecoder
	}
}
