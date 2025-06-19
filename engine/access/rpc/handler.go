package rpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
)

type Handler struct {
	subscription.StreamingData
	api                  access.API
	chain                flow.Chain
	signerIndicesDecoder hotstuff.BlockSignerDecoder
	finalizedHeaderCache module.FinalizedHeaderCache
	me                   module.Local
	indexReporter        state_synchronization.IndexReporter
}

// HandlerOption is used to hand over optional constructor parameters
type HandlerOption func(*Handler)

var _ accessproto.AccessAPIServer = (*Handler)(nil)

// sendSubscribeBlocksResponseFunc is a callback function used to send
// SubscribeBlocksResponse to the client stream.
type sendSubscribeBlocksResponseFunc func(*accessproto.SubscribeBlocksResponse) error

// sendSubscribeBlockHeadersResponseFunc is a callback function used to send
// SubscribeBlockHeadersResponse to the client stream.
type sendSubscribeBlockHeadersResponseFunc func(*accessproto.SubscribeBlockHeadersResponse) error

// sendSubscribeBlockDigestsResponseFunc is a callback function used to send
// SubscribeBlockDigestsResponse to the client stream.
type sendSubscribeBlockDigestsResponseFunc func(*accessproto.SubscribeBlockDigestsResponse) error

func NewHandler(
	api access.API,
	chain flow.Chain,
	finalizedHeader module.FinalizedHeaderCache,
	me module.Local,
	maxStreams uint32,
	options ...HandlerOption,
) *Handler {
	h := &Handler{
		StreamingData:        subscription.NewStreamingData(maxStreams),
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
func (h *Handler) Ping(ctx context.Context, _ *accessproto.PingRequest) (*accessproto.PingResponse, error) {
	err := h.api.Ping(ctx)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return &accessproto.PingResponse{}, nil
}

// GetNodeVersionInfo gets node version information such as semver, commit, sporkID, protocolVersion, etc
func (h *Handler) GetNodeVersionInfo(
	ctx context.Context,
	_ *accessproto.GetNodeVersionInfoRequest,
) (*accessproto.GetNodeVersionInfoResponse, error) {
	nodeVersionInfo, err := h.api.GetNodeVersionInfo(ctx)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return &accessproto.GetNodeVersionInfoResponse{
		Info: &entities.NodeVersionInfo{
			Semver:               nodeVersionInfo.Semver,
			Commit:               nodeVersionInfo.Commit,
			SporkId:              nodeVersionInfo.SporkId[:],
			ProtocolVersion:      nodeVersionInfo.ProtocolVersion,
			ProtocolStateVersion: nodeVersionInfo.ProtocolStateVersion,
			SporkRootBlockHeight: nodeVersionInfo.SporkRootBlockHeight,
			NodeRootBlockHeight:  nodeVersionInfo.NodeRootBlockHeight,
			CompatibleRange:      convert.CompatibleRangeToMessage(nodeVersionInfo.CompatibleRange),
		},
	}, nil
}

func (h *Handler) GetNetworkParameters(
	ctx context.Context,
	_ *accessproto.GetNetworkParametersRequest,
) (*accessproto.GetNetworkParametersResponse, error) {
	params := h.api.GetNetworkParameters(ctx)

	return &accessproto.GetNetworkParametersResponse{
		ChainId: string(params.ChainID),
	}, nil
}

// GetLatestBlockHeader gets the latest sealed block header.
func (h *Handler) GetLatestBlockHeader(
	ctx context.Context,
	req *accessproto.GetLatestBlockHeaderRequest,
) (*accessproto.BlockHeaderResponse, error) {
	header, status, err := h.api.GetLatestBlockHeader(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header, status)
}

// GetBlockHeaderByHeight gets a block header by height.
func (h *Handler) GetBlockHeaderByHeight(
	ctx context.Context,
	req *accessproto.GetBlockHeaderByHeightRequest,
) (*accessproto.BlockHeaderResponse, error) {
	header, status, err := h.api.GetBlockHeaderByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockHeaderResponse(header, status)
}

// GetBlockHeaderByID gets a block header by ID.
func (h *Handler) GetBlockHeaderByID(
	ctx context.Context,
	req *accessproto.GetBlockHeaderByIDRequest,
) (*accessproto.BlockHeaderResponse, error) {
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
	req *accessproto.GetLatestBlockRequest,
) (*accessproto.BlockResponse, error) {
	block, status, err := h.api.GetLatestBlock(ctx, req.GetIsSealed())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse(), status)
}

// GetBlockByHeight gets a block by height.
func (h *Handler) GetBlockByHeight(
	ctx context.Context,
	req *accessproto.GetBlockByHeightRequest,
) (*accessproto.BlockResponse, error) {
	block, status, err := h.api.GetBlockByHeight(ctx, req.GetHeight())
	if err != nil {
		return nil, err
	}
	return h.blockResponse(block, req.GetFullBlockResponse(), status)
}

// GetBlockByID gets a block by ID.
func (h *Handler) GetBlockByID(
	ctx context.Context,
	req *accessproto.GetBlockByIDRequest,
) (*accessproto.BlockResponse, error) {
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
	req *accessproto.GetCollectionByIDRequest,
) (*accessproto.CollectionResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.CollectionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
	}

	col, err := h.api.GetCollectionByID(ctx, id)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	colMsg, err := convert.LightCollectionToMessage(col)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &accessproto.CollectionResponse{
		Collection: colMsg,
		Metadata:   metadata,
	}, nil
}

func (h *Handler) GetFullCollectionByID(
	ctx context.Context,
	req *accessproto.GetFullCollectionByIDRequest,
) (*accessproto.FullCollectionResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.CollectionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid collection id: %v", err)
	}

	col, err := h.api.GetFullCollectionByID(ctx, id)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	transactions, err := convert.FullCollectionToMessage(col)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &accessproto.FullCollectionResponse{
		Transactions: transactions,
		Metadata:     metadata,
	}, nil
}

// SendTransaction submits a transaction to the network.
func (h *Handler) SendTransaction(
	ctx context.Context,
	req *accessproto.SendTransactionRequest,
) (*accessproto.SendTransactionResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	return &accessproto.SendTransactionResponse{
		Id:       txID[:],
		Metadata: metadata,
	}, nil
}

// GetTransaction gets a transaction by ID.
func (h *Handler) GetTransaction(
	ctx context.Context,
	req *accessproto.GetTransactionRequest,
) (*accessproto.TransactionResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.TransactionID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transaction id: %v", err)
	}

	tx, err := h.api.GetTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return &accessproto.TransactionResponse{
		Transaction: convert.TransactionToMessage(*tx),
		Metadata:    metadata,
	}, nil
}

// GetTransactionResult gets a transaction by ID.
func (h *Handler) GetTransactionResult(
	ctx context.Context,
	req *accessproto.GetTransactionRequest,
) (*accessproto.TransactionResultResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	message := convert.TransactionResultToMessage(result)
	message.Metadata = metadata

	return message, nil
}

func (h *Handler) GetTransactionResultsByBlockID(
	ctx context.Context,
	req *accessproto.GetTransactionsByBlockIDRequest,
) (*accessproto.TransactionResultsResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	eventEncodingVersion := req.GetEventEncodingVersion()

	results, err := h.api.GetTransactionResultsByBlockID(ctx, id, eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	message := convert.TransactionResultsToMessage(results)
	message.Metadata = metadata

	return message, nil
}

func (h *Handler) GetSystemTransaction(
	ctx context.Context,
	req *accessproto.GetSystemTransactionRequest,
) (*accessproto.TransactionResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	tx, err := h.api.GetSystemTransaction(ctx, id)
	if err != nil {
		return nil, err
	}

	return &accessproto.TransactionResponse{
		Transaction: convert.TransactionToMessage(*tx),
		Metadata:    metadata,
	}, nil
}

func (h *Handler) GetSystemTransactionResult(
	ctx context.Context,
	req *accessproto.GetSystemTransactionResultRequest,
) (*accessproto.TransactionResultResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	result, err := h.api.GetSystemTransactionResult(ctx, id, req.GetEventEncodingVersion())
	if err != nil {
		return nil, err
	}

	message := convert.TransactionResultToMessage(result)
	message.Metadata = metadata

	return message, nil
}

func (h *Handler) GetTransactionsByBlockID(
	ctx context.Context,
	req *accessproto.GetTransactionsByBlockIDRequest,
) (*accessproto.TransactionsResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	id, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	transactions, err := h.api.GetTransactionsByBlockID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &accessproto.TransactionsResponse{
		Transactions: convert.TransactionsToMessages(transactions),
		Metadata:     metadata,
	}, nil
}

// GetTransactionResultByIndex gets a transaction at a specific index for in a block that is executed,
// pending or finalized transactions return errors
func (h *Handler) GetTransactionResultByIndex(
	ctx context.Context,
	req *accessproto.GetTransactionByIndexRequest,
) (*accessproto.TransactionResultResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	blockID, err := convert.BlockID(req.GetBlockId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid block id: %v", err)
	}

	eventEncodingVersion := req.GetEventEncodingVersion()

	result, err := h.api.GetTransactionResultByIndex(ctx, blockID, req.GetIndex(), eventEncodingVersion)
	if err != nil {
		return nil, err
	}

	message := convert.TransactionResultToMessage(result)
	message.Metadata = metadata

	return message, nil
}

// GetAccount returns an account by address at the latest sealed block.
func (h *Handler) GetAccount(
	ctx context.Context,
	req *accessproto.GetAccountRequest,
) (*accessproto.GetAccountResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	return &accessproto.GetAccountResponse{
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

// GetAccountAtLatestBlock returns an account by address at the latest sealed block.
func (h *Handler) GetAccountAtLatestBlock(
	ctx context.Context,
	req *accessproto.GetAccountAtLatestBlockRequest,
) (*accessproto.AccountResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	return &accessproto.AccountResponse{
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

// GetAccountAtBlockHeight returns an account by address at the given block height.
func (h *Handler) GetAccountAtBlockHeight(
	ctx context.Context,
	req *accessproto.GetAccountAtBlockHeightRequest,
) (*accessproto.AccountResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	return &accessproto.AccountResponse{
		Account:  accountMsg,
		Metadata: metadata,
	}, nil
}

// GetAccountBalanceAtLatestBlock returns an account balance by address at the latest sealed block.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node or failed to convert account message.
func (h *Handler) GetAccountBalanceAtLatestBlock(
	ctx context.Context,
	req *accessproto.GetAccountBalanceAtLatestBlockRequest,
) (*accessproto.AccountBalanceResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	accountBalance, err := h.api.GetAccountBalanceAtLatestBlock(ctx, address)
	if err != nil {
		return nil, err
	}

	return &accessproto.AccountBalanceResponse{
		Balance:  accountBalance,
		Metadata: metadata,
	}, nil
}

// GetAccountBalanceAtBlockHeight returns an account balance by address at the given block height.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node or failed to convert account message.

func (h *Handler) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	req *accessproto.GetAccountBalanceAtBlockHeightRequest,
) (*accessproto.AccountBalanceResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	accountBalance, err := h.api.GetAccountBalanceAtBlockHeight(ctx, address, req.GetBlockHeight())
	if err != nil {
		return nil, err
	}

	return &accessproto.AccountBalanceResponse{
		Balance:  accountBalance,
		Metadata: metadata,
	}, nil
}

// GetAccountKeyAtLatestBlock returns an account public key by address and key index at the latest sealed block.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node, ailed to convert account message or failed to encode account key.
func (h *Handler) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	req *accessproto.GetAccountKeyAtLatestBlockRequest,
) (*accessproto.AccountKeyResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	keyByIndex, err := h.api.GetAccountKeyAtLatestBlock(ctx, address, req.GetIndex())
	if err != nil {
		return nil, err
	}

	accountKey, err := convert.AccountKeyToMessage(*keyByIndex)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to encode account key: %v", err)
	}

	return &accessproto.AccountKeyResponse{
		AccountKey: accountKey,
		Metadata:   metadata,
	}, nil
}

// GetAccountKeysAtLatestBlock returns an account public keys by address at the latest sealed block.
// GetAccountKeyAtLatestBlock returns an account public key by address and key index at the latest sealed block.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node, ailed to convert account message or failed to encode account key.
func (h *Handler) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	req *accessproto.GetAccountKeysAtLatestBlockRequest,
) (*accessproto.AccountKeysResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	accountKeys, err := h.api.GetAccountKeysAtLatestBlock(ctx, address)
	if err != nil {
		return nil, err
	}

	var publicKeys []*entities.AccountKey

	for i, key := range accountKeys {
		accountKey, err := convert.AccountKeyToMessage(key)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to encode account key %d: %v", i, err)
		}

		publicKeys = append(publicKeys, accountKey)
	}

	return &accessproto.AccountKeysResponse{
		AccountKeys: publicKeys,
		Metadata:    metadata,
	}, nil
}

// GetAccountKeyAtBlockHeight returns an account public keys by address and key index at the given block height.
// GetAccountKeyAtLatestBlock returns an account public key by address and key index at the latest sealed block.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node, ailed to convert account message or failed to encode account key.
func (h *Handler) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	req *accessproto.GetAccountKeyAtBlockHeightRequest,
) (*accessproto.AccountKeyResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	keyByIndex, err := h.api.GetAccountKeyAtBlockHeight(ctx, address, req.GetIndex(), req.GetBlockHeight())
	if err != nil {
		return nil, err
	}

	accountKey, err := convert.AccountKeyToMessage(*keyByIndex)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to encode account key: %v", err)
	}

	return &accessproto.AccountKeyResponse{
		AccountKey: accountKey,
		Metadata:   metadata,
	}, nil
}

// GetAccountKeysAtBlockHeight returns an account public keys by address at the given block height.
// GetAccountKeyAtLatestBlock returns an account public key by address and key index at the latest sealed block.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid account address provided.
// - codes.Internal - if failed to get account from the execution node, ailed to convert account message or failed to encode account key.
func (h *Handler) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	req *accessproto.GetAccountKeysAtBlockHeightRequest,
) (*accessproto.AccountKeysResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	address, err := convert.Address(req.GetAddress(), h.chain)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address: %v", err)
	}

	accountKeys, err := h.api.GetAccountKeysAtBlockHeight(ctx, address, req.GetBlockHeight())
	if err != nil {
		return nil, err
	}

	var publicKeys []*entities.AccountKey

	for i, key := range accountKeys {
		accountKey, err := convert.AccountKeyToMessage(key)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to encode account key %d: %v", i, err)
		}

		publicKeys = append(publicKeys, accountKey)
	}

	return &accessproto.AccountKeysResponse{
		AccountKeys: publicKeys,
		Metadata:    metadata,
	}, nil
}

// ExecuteScriptAtLatestBlock executes a script at a the latest block.
func (h *Handler) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	req *accessproto.ExecuteScriptAtLatestBlockRequest,
) (*accessproto.ExecuteScriptResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	script := req.GetScript()
	arguments := req.GetArguments()

	value, err := h.api.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	if err != nil {
		return nil, err
	}

	return &accessproto.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// ExecuteScriptAtBlockHeight executes a script at a specific block height.
func (h *Handler) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	req *accessproto.ExecuteScriptAtBlockHeightRequest,
) (*accessproto.ExecuteScriptResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	script := req.GetScript()
	arguments := req.GetArguments()
	blockHeight := req.GetBlockHeight()

	value, err := h.api.ExecuteScriptAtBlockHeight(ctx, blockHeight, script, arguments)
	if err != nil {
		return nil, err
	}

	return &accessproto.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// ExecuteScriptAtBlockID executes a script at a specific block ID.
func (h *Handler) ExecuteScriptAtBlockID(
	ctx context.Context,
	req *accessproto.ExecuteScriptAtBlockIDRequest,
) (*accessproto.ExecuteScriptResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}
	script := req.GetScript()
	arguments := req.GetArguments()
	blockID := convert.MessageToIdentifier(req.GetBlockId())

	value, err := h.api.ExecuteScriptAtBlockID(ctx, blockID, script, arguments)
	if err != nil {
		return nil, err
	}

	return &accessproto.ExecuteScriptResponse{
		Value:    value,
		Metadata: metadata,
	}, nil
}

// GetEventsForHeightRange returns events matching a query.
func (h *Handler) GetEventsForHeightRange(
	ctx context.Context,
	req *accessproto.GetEventsForHeightRangeRequest,
) (*accessproto.EventsResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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
	return &accessproto.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

// GetEventsForBlockIDs returns events matching a set of block IDs.
func (h *Handler) GetEventsForBlockIDs(
	ctx context.Context,
	req *accessproto.GetEventsForBlockIDsRequest,
) (*accessproto.EventsResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

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

	return &accessproto.EventsResponse{
		Results:  resultEvents,
		Metadata: metadata,
	}, nil
}

// GetLatestProtocolStateSnapshot returns the latest serializable Snapshot
func (h *Handler) GetLatestProtocolStateSnapshot(ctx context.Context, req *accessproto.GetLatestProtocolStateSnapshotRequest) (*accessproto.ProtocolStateSnapshotResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	snapshot, err := h.api.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return &accessproto.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
		Metadata:           metadata,
	}, nil
}

// GetProtocolStateSnapshotByBlockID returns serializable Snapshot by blockID
func (h *Handler) GetProtocolStateSnapshotByBlockID(ctx context.Context, req *accessproto.GetProtocolStateSnapshotByBlockIDRequest) (*accessproto.ProtocolStateSnapshotResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	blockID := convert.MessageToIdentifier(req.GetBlockId())

	snapshot, err := h.api.GetProtocolStateSnapshotByBlockID(ctx, blockID)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return &accessproto.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
		Metadata:           metadata,
	}, nil
}

// GetProtocolStateSnapshotByHeight returns serializable Snapshot by block height
func (h *Handler) GetProtocolStateSnapshotByHeight(ctx context.Context, req *accessproto.GetProtocolStateSnapshotByHeightRequest) (*accessproto.ProtocolStateSnapshotResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	snapshot, err := h.api.GetProtocolStateSnapshotByHeight(ctx, req.GetBlockHeight())
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return &accessproto.ProtocolStateSnapshotResponse{
		SerializedSnapshot: snapshot,
		Metadata:           metadata,
	}, nil
}

// GetExecutionResultForBlockID returns the latest received execution result for the given block ID.
// AN might receive multiple receipts with conflicting results for unsealed blocks.
// If this case happens, since AN is not able to determine which result is the correct one until the block is sealed, it has to pick one result to respond to this query. For now, we return the result from the latest received receipt.
func (h *Handler) GetExecutionResultForBlockID(ctx context.Context, req *accessproto.GetExecutionResultForBlockIDRequest) (*accessproto.ExecutionResultForBlockIDResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	blockID := convert.MessageToIdentifier(req.GetBlockId())

	result, err := h.api.GetExecutionResultForBlockID(ctx, blockID)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	return executionResultToMessages(result, metadata)
}

// GetExecutionResultByID returns the execution result for the given ID.
func (h *Handler) GetExecutionResultByID(ctx context.Context, req *accessproto.GetExecutionResultByIDRequest) (*accessproto.ExecutionResultByIDResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	resultID := convert.MessageToIdentifier(req.GetId())

	result, err := h.api.GetExecutionResultByID(ctx, resultID)
	if err != nil {
		return nil, rpc.ErrorToStatus(err)
	}

	execResult, err := convert.ExecutionResultToMessage(result)
	if err != nil {
		return nil, err
	}
	return &accessproto.ExecutionResultByIDResponse{
		ExecutionResult: execResult,
		Metadata:        metadata,
	}, nil
}

// SubscribeBlocksFromStartBlockID handles subscription requests for blocks started from block id.
// It takes a SubscribeBlocksFromStartBlockIDRequest and an AccessAPI_SubscribeBlocksFromStartBlockIDServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid startBlockID provided or unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlocksFromStartBlockID(request *accessproto.SubscribeBlocksFromStartBlockIDRequest, stream accessproto.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	startBlockID, blockStatus, err := h.getSubscriptionDataFromStartBlockID(request.GetStartBlockId(), request.GetBlockStatus())
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlocksFromStartBlockID(stream.Context(), startBlockID, blockStatus)
	return HandleRPCSubscription(sub, h.handleBlocksResponse(stream.Send, request.GetFullBlockResponse(), blockStatus))
}

// SubscribeBlocksFromStartHeight handles subscription requests for blocks started from block height.
// It takes a SubscribeBlocksFromStartHeightRequest and an AccessAPI_SubscribeBlocksFromStartHeightServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlocksFromStartHeight(request *accessproto.SubscribeBlocksFromStartHeightRequest, stream accessproto.AccessAPI_SubscribeBlocksFromStartHeightServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlocksFromStartHeight(stream.Context(), request.GetStartBlockHeight(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlocksResponse(stream.Send, request.GetFullBlockResponse(), blockStatus))
}

// SubscribeBlocksFromLatest handles subscription requests for blocks started from latest sealed block.
// It takes a SubscribeBlocksFromLatestRequest and an AccessAPI_SubscribeBlocksFromLatestServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlocksFromLatest(request *accessproto.SubscribeBlocksFromLatestRequest, stream accessproto.AccessAPI_SubscribeBlocksFromLatestServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlocksFromLatest(stream.Context(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlocksResponse(stream.Send, request.GetFullBlockResponse(), blockStatus))
}

// handleBlocksResponse handles the subscription to block updates and sends
// the subscribed block information to the client via the provided stream.
//
// Parameters:
// - send: The function responsible for sending the block response to the client.
// - fullBlockResponse: A boolean indicating whether to include full block responses.
// - blockStatus: The current block status.
//
// Returns a function that can be used as a callback for block updates.
//
// This function is designed to be used as a callback for block updates in a subscription.
// It takes a block, processes it, and sends the corresponding response to the client using the provided send function.
//
// Expected errors during normal operation:
//   - codes.Internal: If cannot convert a block to a message or the stream could not send a response.
func (h *Handler) handleBlocksResponse(send sendSubscribeBlocksResponseFunc, fullBlockResponse bool, blockStatus flow.BlockStatus) func(*flow.Block) error {
	return func(block *flow.Block) error {
		msgBlockResponse, err := h.blockResponse(block, fullBlockResponse, blockStatus)
		if err != nil {
			return rpc.ConvertError(err, "could not convert block to message", codes.Internal)
		}

		err = send(&accessproto.SubscribeBlocksResponse{
			Block: msgBlockResponse.Block,
		})
		if err != nil {
			return rpc.ConvertError(err, "could not send response", codes.Internal)
		}

		return nil
	}
}

// SubscribeBlockHeadersFromStartBlockID handles subscription requests for block headers started from block id.
// It takes a SubscribeBlockHeadersFromStartBlockIDRequest and an AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block header information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid startBlockID provided or unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block header to message or could not send response.
func (h *Handler) SubscribeBlockHeadersFromStartBlockID(request *accessproto.SubscribeBlockHeadersFromStartBlockIDRequest, stream accessproto.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	startBlockID, blockStatus, err := h.getSubscriptionDataFromStartBlockID(request.GetStartBlockId(), request.GetBlockStatus())
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockHeadersFromStartBlockID(stream.Context(), startBlockID, blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockHeadersResponse(stream.Send))
}

// SubscribeBlockHeadersFromStartHeight handles subscription requests for block headers started from block height.
// It takes a SubscribeBlockHeadersFromStartHeightRequest and an AccessAPI_SubscribeBlockHeadersFromStartHeightServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block header information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block header to message or could not send response.
func (h *Handler) SubscribeBlockHeadersFromStartHeight(request *accessproto.SubscribeBlockHeadersFromStartHeightRequest, stream accessproto.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockHeadersFromStartHeight(stream.Context(), request.GetStartBlockHeight(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockHeadersResponse(stream.Send))
}

// SubscribeBlockHeadersFromLatest handles subscription requests for block headers started from latest sealed block.
// It takes a SubscribeBlockHeadersFromLatestRequest and an AccessAPI_SubscribeBlockHeadersFromLatestServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block header information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block header to message or could not send response.
func (h *Handler) SubscribeBlockHeadersFromLatest(request *accessproto.SubscribeBlockHeadersFromLatestRequest, stream accessproto.AccessAPI_SubscribeBlockHeadersFromLatestServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockHeadersFromLatest(stream.Context(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockHeadersResponse(stream.Send))
}

// handleBlockHeadersResponse handles the subscription to block updates and sends
// the subscribed block header information to the client via the provided stream.
//
// Parameters:
// - send: The function responsible for sending the block header response to the client.
//
// Returns a function that can be used as a callback for block header updates.
//
// This function is designed to be used as a callback for block header updates in a subscription.
// It takes a block header, processes it, and sends the corresponding response to the client using the provided send function.
//
// Expected errors during normal operation:
//   - codes.Internal: If could not decode the signer indices from the given block header, could not convert a block header to a message or the stream could not send a response.
func (h *Handler) handleBlockHeadersResponse(send sendSubscribeBlockHeadersResponseFunc) func(*flow.Header) error {
	return func(header *flow.Header) error {
		signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(header)
		if err != nil {
			return rpc.ConvertError(err, "could not decode the signer indices from the given block header", codes.Internal) // the block was retrieved from local storage - so no errors are expected
		}

		msgHeader, err := convert.BlockHeaderToMessage(header, signerIDs)
		if err != nil {
			return rpc.ConvertError(err, "could not convert block header to message", codes.Internal)
		}

		err = send(&accessproto.SubscribeBlockHeadersResponse{
			Header: msgHeader,
		})
		if err != nil {
			return rpc.ConvertError(err, "could not send response", codes.Internal)
		}

		return nil
	}
}

// SubscribeBlockDigestsFromStartBlockID streams finalized or sealed lightweight block starting at the requested block id.
// It takes a SubscribeBlockDigestsFromStartBlockIDRequest and an AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer stream as input.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if invalid startBlockID provided or unknown block status provided,
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlockDigestsFromStartBlockID(request *accessproto.SubscribeBlockDigestsFromStartBlockIDRequest, stream accessproto.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	startBlockID, blockStatus, err := h.getSubscriptionDataFromStartBlockID(request.GetStartBlockId(), request.GetBlockStatus())
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockDigestsFromStartBlockID(stream.Context(), startBlockID, blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockDigestsResponse(stream.Send))
}

// SubscribeBlockDigestsFromStartHeight handles subscription requests for lightweight blocks started from block height.
// It takes a SubscribeBlockDigestsFromStartHeightRequest and an AccessAPI_SubscribeBlockDigestsFromStartHeightServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlockDigestsFromStartHeight(request *accessproto.SubscribeBlockDigestsFromStartHeightRequest, stream accessproto.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockDigestsFromStartHeight(stream.Context(), request.GetStartBlockHeight(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockDigestsResponse(stream.Send))
}

// SubscribeBlockDigestsFromLatest handles subscription requests for lightweight block started from latest sealed block.
// It takes a SubscribeBlockDigestsFromLatestRequest and an AccessAPI_SubscribeBlockDigestsFromLatestServer stream as input.
// The handler manages the subscription to block updates and sends the subscribed block header information
// to the client via the provided stream.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if unknown block status provided.
// - codes.ResourceExhausted - if the maximum number of streams is reached.
// - codes.Internal - if stream encountered an error, if stream got unexpected response or could not convert block to message or could not send response.
func (h *Handler) SubscribeBlockDigestsFromLatest(request *accessproto.SubscribeBlockDigestsFromLatestRequest, stream accessproto.AccessAPI_SubscribeBlockDigestsFromLatestServer) error {
	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	blockStatus := convert.MessageToBlockStatus(request.GetBlockStatus())
	err := checkBlockStatus(blockStatus)
	if err != nil {
		return err
	}

	sub := h.api.SubscribeBlockDigestsFromLatest(stream.Context(), blockStatus)
	return HandleRPCSubscription(sub, h.handleBlockDigestsResponse(stream.Send))
}

// handleBlockDigestsResponse handles the subscription to block updates and sends
// the subscribed block digest information to the client via the provided stream.
//
// Parameters:
// - send: The function responsible for sending the block digest response to the client.
//
// Returns a function that can be used as a callback for block digest updates.
//
// This function is designed to be used as a callback for block digest updates in a subscription.
// It takes a block digest, processes it, and sends the corresponding response to the client using the provided send function.
//
// Expected errors during normal operation:
//   - codes.Internal: if the stream cannot send a response.
func (h *Handler) handleBlockDigestsResponse(send sendSubscribeBlockDigestsResponseFunc) func(*flow.BlockDigest) error {
	return func(blockDigest *flow.BlockDigest) error {
		err := send(&accessproto.SubscribeBlockDigestsResponse{
			BlockId:        convert.IdentifierToMessage(blockDigest.ID()),
			BlockHeight:    blockDigest.Height,
			BlockTimestamp: timestamppb.New(blockDigest.Timestamp),
		})
		if err != nil {
			return rpc.ConvertError(err, "could not send response", codes.Internal)
		}

		return nil
	}
}

// getSubscriptionDataFromStartBlockID processes subscription start data from start block id.
// It takes a union representing the start block id and a BlockStatus from the entities package.
// Performs validation of input data and returns it in expected format for further processing.
//
// Returns:
// - flow.Identifier: The start block id for searching.
// - flow.BlockStatus: Block status.
// - error: An error indicating the result of the operation, if any.
//
// Expected errors during normal operation:
// - codes.InvalidArgument: If blockStatus is flow.BlockStatusUnknown, or startBlockID could not convert to flow.Identifier.
func (h *Handler) getSubscriptionDataFromStartBlockID(msgBlockId []byte, msgBlockStatus entities.BlockStatus) (flow.Identifier, flow.BlockStatus, error) {
	startBlockID, err := convert.BlockID(msgBlockId)
	if err != nil {
		return flow.ZeroID, flow.BlockStatusUnknown, err
	}

	blockStatus := convert.MessageToBlockStatus(msgBlockStatus)
	err = checkBlockStatus(blockStatus)
	if err != nil {
		return flow.ZeroID, flow.BlockStatusUnknown, err
	}

	return startBlockID, blockStatus, nil
}

// SendAndSubscribeTransactionStatuses streams transaction statuses starting from the reference block saved in the
// transaction itself until the block containing the transaction becomes sealed or expired. When the transaction
// status becomes TransactionStatusSealed or TransactionStatusExpired, the subscription will automatically shut down.
func (h *Handler) SendAndSubscribeTransactionStatuses(
	request *accessproto.SendAndSubscribeTransactionStatusesRequest,
	stream accessproto.AccessAPI_SendAndSubscribeTransactionStatusesServer,
) error {
	ctx := stream.Context()

	// check if the maximum number of streams is reached
	if h.StreamCount.Load() >= h.MaxStreams {
		return status.Errorf(codes.ResourceExhausted, "maximum number of streams reached")
	}
	h.StreamCount.Add(1)
	defer h.StreamCount.Add(-1)

	tx, err := convert.MessageToTransaction(request.GetTransaction(), h.chain)
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	sub := h.api.SendAndSubscribeTransactionStatuses(ctx, &tx, request.GetEventEncodingVersion())

	messageIndex := counters.NewMonotonicCounter(0)
	return HandleRPCSubscription(sub, func(txResults []*accessmodel.TransactionResult) error {
		for i := range txResults {
			index := messageIndex.Value()
			if ok := messageIndex.Set(index + 1); !ok {
				return status.Errorf(codes.Internal, "message index already incremented to %d", messageIndex.Value())
			}

			err = stream.Send(&accessproto.SendAndSubscribeTransactionStatusesResponse{
				TransactionResults: convert.TransactionResultToMessage(txResults[i]),
				MessageIndex:       index,
			})
			if err != nil {
				return rpc.ConvertError(err, "could not send response", codes.Internal)
			}

		}

		return nil
	})
}

func (h *Handler) blockResponse(block *flow.Block, fullResponse bool, status flow.BlockStatus) (*accessproto.BlockResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(block.Header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	var msg *entities.Block
	if fullResponse {
		msg, err = convert.BlockToMessage(block, signerIDs)
		if err != nil {
			return nil, rpc.ConvertError(err, "could not convert block to message", codes.Internal)
		}
	} else {
		msg = convert.BlockToMessageLight(block)
	}

	return &accessproto.BlockResponse{
		Block:       msg,
		BlockStatus: entities.BlockStatus(status),
		Metadata:    metadata,
	}, nil
}

func (h *Handler) blockHeaderResponse(header *flow.Header, status flow.BlockStatus) (*accessproto.BlockHeaderResponse, error) {
	metadata, err := h.buildMetadataResponse()
	if err != nil {
		return nil, err
	}

	signerIDs, err := h.signerIndicesDecoder.DecodeSignerIDs(header)
	if err != nil {
		return nil, err // the block was retrieved from local storage - so no errors are expected
	}

	msg, err := convert.BlockHeaderToMessage(header, signerIDs)
	if err != nil {
		return nil, rpc.ConvertError(err, "could not convert block header to message", codes.Internal)
	}

	return &accessproto.BlockHeaderResponse{
		Block:       msg,
		BlockStatus: entities.BlockStatus(status),
		Metadata:    metadata,
	}, nil
}

// buildMetadataResponse builds and returns the metadata response object.
// Expected errors during normal operation:
//   - codes.NotFound if result cannot be provided by storage due to the absence of data.
//   - storage.ErrHeightNotIndexed when data is unavailable
func (h *Handler) buildMetadataResponse() (*entities.Metadata, error) {
	lastFinalizedHeader := h.finalizedHeaderCache.Get()
	blockId := lastFinalizedHeader.ID()
	nodeId := h.me.NodeID()

	metadata := &entities.Metadata{
		LatestFinalizedBlockId: blockId[:],
		LatestFinalizedHeight:  lastFinalizedHeader.Height,
		NodeId:                 nodeId[:],
	}

	if h.indexReporter != nil {
		highestIndexedHeight, err := h.indexReporter.HighestIndexedHeight()
		if err != nil {
			if !errors.Is(err, indexer.ErrIndexNotInitialized) {
				return nil, rpc.ConvertIndexError(err, lastFinalizedHeader.Height, "could not get highest indexed height")
			}
			highestIndexedHeight = 0
		}

		metadata.HighestIndexedHeight = highestIndexedHeight
	}

	return metadata, nil
}

func executionResultToMessages(er *flow.ExecutionResult, metadata *entities.Metadata) (*accessproto.ExecutionResultForBlockIDResponse, error) {
	execResult, err := convert.ExecutionResultToMessage(er)
	if err != nil {
		return nil, err
	}
	return &accessproto.ExecutionResultForBlockIDResponse{
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

// WithIndexReporter configures the Handler to work with index reporter
func WithIndexReporter(indexReporter state_synchronization.IndexReporter) func(*Handler) {
	return func(handler *Handler) {
		handler.indexReporter = indexReporter
	}
}

// checkBlockStatus checks the validity of the provided block status.
//
// Expected errors during normal operation:
// - codes.InvalidArgument - if blockStatus is flow.BlockStatusUnknown
func checkBlockStatus(blockStatus flow.BlockStatus) error {
	if blockStatus != flow.BlockStatusFinalized && blockStatus != flow.BlockStatusSealed {
		return status.Errorf(codes.InvalidArgument, "block status is unknown. Possible variants: BLOCK_FINALIZED, BLOCK_SEALED")
	}
	return nil
}

// HandleRPCSubscription is a generic handler for subscriptions to a specific type for rpc calls.
//
// Parameters:
// - sub: The subscription.
// - handleResponse: The function responsible for handling the response of the subscribed type.
//
// Expected errors during normal operation:
//   - codes.Internal: If the subscription encounters an error or gets an unexpected response.
func HandleRPCSubscription[T any](sub subscription.Subscription, handleResponse func(resp T) error) error {
	err := subscription.HandleSubscription(sub, handleResponse)
	if err != nil {
		return rpc.ConvertError(err, "handle subscription error", codes.Internal)
	}

	return nil
}
