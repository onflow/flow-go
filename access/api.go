package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

type AccountsAPI interface {
	GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error)
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error)
	GetAccountBalanceAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (uint64, error)

	GetAccountKeyAtLatestBlock(ctx context.Context, address flow.Address, keyIndex uint32) (*flow.AccountPublicKey, error)
	GetAccountKeyAtBlockHeight(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error)
	GetAccountKeysAtLatestBlock(ctx context.Context, address flow.Address) ([]flow.AccountPublicKey, error)
	GetAccountKeysAtBlockHeight(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error)
}

type EventsAPI interface {
	GetEventsForHeightRange(
		ctx context.Context,
		eventType string,
		startHeight,
		endHeight uint64,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) ([]flow.BlockEvents, error)

	GetEventsForBlockIDs(
		ctx context.Context,
		eventType string,
		blockIDs []flow.Identifier,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) ([]flow.BlockEvents, error)
}

type ScriptsAPI interface {
	ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error)
	ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error)
}

type TransactionsAPI interface {
	SendTransaction(ctx context.Context, tx *flow.TransactionBody) error

	GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error)
	GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error)

	GetTransactionResult(ctx context.Context, txID flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
	GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
	GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) ([]*accessmodel.TransactionResult, error)

	GetSystemTransaction(ctx context.Context, blockID flow.Identifier) (*flow.TransactionBody, error)
	GetSystemTransactionResult(ctx context.Context, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
}

type TransactionStreamAPI interface {
	// SubscribeTransactionStatuses subscribes to transaction status updates for a given transaction ID. Monitoring starts
	// from the latest block to obtain the current transaction status. If the transaction is already in the final state
	// ([flow.TransactionStatusSealed] or [flow.TransactionStatusExpired]), all statuses will be prepared and sent to the client
	// sequentially. If the transaction is not in the final state, the subscription will stream status updates until the transaction
	// reaches the final state. Once a final state is reached, the subscription will automatically terminate.
	//
	// Parameters:
	//   - ctx: Context to manage the subscription's lifecycle, including cancellation.
	//   - txID: The unique identifier of the transaction to monitor.
	//   - requiredEventEncodingVersion: The version of event encoding required for the subscription.
	SubscribeTransactionStatuses(
		ctx context.Context,
		txID flow.Identifier,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) subscription.Subscription

	// SendAndSubscribeTransactionStatuses sends a transaction to the execution node and subscribes to its status updates.
	// Monitoring begins from the reference block saved in the transaction itself and streams status updates until the transaction
	// reaches the final state ([flow.TransactionStatusSealed] or [flow.TransactionStatusExpired]). Once the final status has been reached, the subscription
	// automatically terminates.
	//
	// Parameters:
	//   - ctx: The context to manage the transaction sending and subscription lifecycle, including cancellation.
	//   - tx: The transaction body to be sent and monitored.
	//   - requiredEventEncodingVersion: The version of event encoding required for the subscription.
	//
	// If the transaction cannot be sent, the subscription will fail and return a failed subscription.
	SendAndSubscribeTransactionStatuses(
		ctx context.Context,
		tx *flow.TransactionBody,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) subscription.Subscription
}

// API provides all public-facing functionality of the Flow Access API.
type API interface {
	AccountsAPI
	EventsAPI
	ScriptsAPI
	TransactionsAPI
	TransactionStreamAPI

	Ping(ctx context.Context) error
	GetNetworkParameters(ctx context.Context) accessmodel.NetworkParameters
	GetNodeVersionInfo(ctx context.Context) (*accessmodel.NodeVersionInfo, error)

	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error)
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error)
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error)

	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error)
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error)

	GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error)
	GetFullCollectionByID(ctx context.Context, id flow.Identifier) (*flow.Collection, error)

	GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error)
	GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error)
	GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error)

	GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error)
	GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error)

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
