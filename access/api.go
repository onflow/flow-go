package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

// API provides all public-facing functionality of the Flow Access API.
type API interface {
	Ping(ctx context.Context) error
	GetNetworkParameters(ctx context.Context) NetworkParameters
	GetNodeVersionInfo(ctx context.Context) (*NodeVersionInfo, error)

	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error)

	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error)

	GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error)

	SendTransaction(ctx context.Context, tx *flow.TransactionBody) error
	GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error)
	GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error)
	GetTransactionResult(ctx context.Context, id flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error)
	GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error)
	GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]*TransactionResult, error)
	GetSystemTransaction(ctx context.Context, blockID flow.Identifier) (*flow.TransactionBody, error)
	GetSystemTransactionResult(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*TransactionResult, error)

	GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error)

	GetEventsForHeightRange(ctx context.Context, eventType string, startHeight, endHeight uint64, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error)
	GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error)

	GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error)
	GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error)
	GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error)

	GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error)
	GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error)

	// SubscribeBlocks

	// SubscribeBlocksFromStartBlockID subscribes to the finalized or sealed blocks starting at the requested
	// start block id, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlocksFromStartBlockID will return a failed subscription.
	SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlocksFromStartHeight subscribes to the finalized or sealed blocks starting at the requested
	// start block height, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startHeight: The height of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlocksFromStartHeight will return a failed subscription.
	SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlocksFromLatest subscribes to the finalized or sealed blocks starting at the latest sealed block,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlocksFromLatest will return a failed subscription.
	SubscribeBlocksFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeHeaders

	// SubscribeBlockHeadersFromStartBlockID streams finalized or sealed block headers starting at the requested
	// start block id, up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block header are filtered by the provided block status, and only
	// those block headers that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockHeadersFromStartBlockID will return a failed subscription.
	SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlockHeadersFromStartHeight streams finalized or sealed block headers starting at the requested
	// start block height, up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block header are filtered by the provided block status, and only
	// those block headers that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startHeight: The height of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockHeadersFromStartHeight will return a failed subscription.
	SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlockHeadersFromLatest streams finalized or sealed block headers starting at the latest sealed block,
	// up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block header are filtered by the provided block status, and only
	// those block headers that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockHeadersFromLatest will return a failed subscription.
	SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription

	// Subscribe digests

	// SubscribeBlockDigestsFromStartBlockID streams finalized or sealed lightweight block starting at the requested
	// start block id, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each lightweight block are filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockDigestsFromStartBlockID will return a failed subscription.
	SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlockDigestsFromStartHeight streams finalized or sealed lightweight block starting at the requested
	// start block height, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each lightweight block are filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startHeight: The height of the starting block.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockDigestsFromStartHeight will return a failed subscription.
	SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription
	// SubscribeBlockDigestsFromLatest streams finalized or sealed lightweight block starting at the latest sealed block,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each lightweight block are filtered by the provided block status, and only
	// those blocks that match the status are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - blockStatus: The status of the block, which could be only BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters will be supplied SubscribeBlockDigestsFromLatest will return a failed subscription.
	SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription
}

// TODO: Combine this with flow.TransactionResult?
type TransactionResult struct {
	Status        flow.TransactionStatus
	StatusCode    uint
	Events        []flow.Event
	ErrorMessage  string
	BlockID       flow.Identifier
	TransactionID flow.Identifier
	CollectionID  flow.Identifier
	BlockHeight   uint64
}

func TransactionResultToMessage(result *TransactionResult) *access.TransactionResultResponse {
	return &access.TransactionResultResponse{
		Status:        entities.TransactionStatus(result.Status),
		StatusCode:    uint32(result.StatusCode),
		ErrorMessage:  result.ErrorMessage,
		Events:        convert.EventsToMessages(result.Events),
		BlockId:       result.BlockID[:],
		TransactionId: result.TransactionID[:],
		CollectionId:  result.CollectionID[:],
		BlockHeight:   result.BlockHeight,
	}
}

func TransactionResultsToMessage(results []*TransactionResult) *access.TransactionResultsResponse {
	messages := make([]*access.TransactionResultResponse, len(results))
	for i, result := range results {
		messages[i] = TransactionResultToMessage(result)
	}

	return &access.TransactionResultsResponse{
		TransactionResults: messages,
	}
}

func MessageToTransactionResult(message *access.TransactionResultResponse) *TransactionResult {

	return &TransactionResult{
		Status:        flow.TransactionStatus(message.Status),
		StatusCode:    uint(message.StatusCode),
		ErrorMessage:  message.ErrorMessage,
		Events:        convert.MessagesToEvents(message.Events),
		BlockID:       flow.HashToID(message.BlockId),
		TransactionID: flow.HashToID(message.TransactionId),
		CollectionID:  flow.HashToID(message.CollectionId),
		BlockHeight:   message.BlockHeight,
	}
}

// NetworkParameters contains the network-wide parameters for the Flow blockchain.
type NetworkParameters struct {
	ChainID flow.ChainID
}

// NodeVersionInfo contains information about node, such as semver, commit, sporkID, protocolVersion, etc
type NodeVersionInfo struct {
	Semver               string
	Commit               string
	SporkId              flow.Identifier
	ProtocolVersion      uint64
	SporkRootBlockHeight uint64
	NodeRootBlockHeight  uint64
}
