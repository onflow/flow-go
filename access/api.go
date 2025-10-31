package access

import (
	"context"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
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
		startHeight uint64,
		endHeight uint64,
		requiredEventEncodingVersion entities.EventEncodingVersion,
		criteria optimistic_sync.Criteria,
	) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error)

	GetEventsForBlockIDs(
		ctx context.Context,
		eventType string,
		blockIDs []flow.Identifier,
		requiredEventEncodingVersion entities.EventEncodingVersion,
		criteria optimistic_sync.Criteria,
	) ([]flow.BlockEvents, *accessmodel.ExecutorMetadata, error)
}

type ScriptsAPI interface {
	// ExecuteScriptAtLatestBlock executes a script at latest block.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.InvalidRequestError] - if the request had invalid arguments.
	//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
	//   - [access.DataNotFoundError] - if data required to process the request is not available.
	//   - [access.OutOfRangeError] - if the requested data is outside the available range.
	//   - [access.PreconditionFailedError] - if data for block is not available.
	//   - [access.RequestCanceledError] - if the script execution was canceled.
	//   - [access.RequestTimedOutError] - if the script execution timed out.
	//   - [access.ServiceUnavailable] - if configured to use an external node for script execution and
	//     no upstream server is available.
	//   - [access.InternalError] - for internal failures or index conversion errors.
	ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte, criteria optimistic_sync.Criteria) ([]byte, *accessmodel.ExecutorMetadata, error)

	// ExecuteScriptAtBlockHeight executes a script at the given block height.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.InvalidRequestError] - if the request had invalid arguments.
	//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
	//   - [access.DataNotFoundError] - if data required to process the request is not available.
	//   - [access.OutOfRangeError] - if the requested data is outside the available range.
	//   - [access.PreconditionFailedError] - if data for block is not available.
	//   - [access.RequestCanceledError] - if the script execution was canceled.
	//   - [access.RequestTimedOutError] - if the script execution timed out.
	//   - [access.ServiceUnavailable] - if configured to use an external node for script execution and
	//     no upstream server is available.
	//   - [access.InternalError] - for internal failures or index conversion errors.
	ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte, criteria optimistic_sync.Criteria) ([]byte, *accessmodel.ExecutorMetadata, error)

	// ExecuteScriptAtBlockID executes a script at the given block id.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.InvalidRequestError] - if the request had invalid arguments.
	//   - [access.ResourceExhausted] - if computation or memory limits were exceeded.
	//   - [access.DataNotFoundError] - if data required to process the request is not available.
	//   - [access.OutOfRangeError] - if the requested data is outside the available range.
	//   - [access.PreconditionFailedError] - if data for block is not available.
	//   - [access.RequestCanceledError] - if the script execution was canceled.
	//   - [access.RequestTimedOutError] - if the script execution timed out.
	//   - [access.ServiceUnavailable] - if configured to use an external node for script execution and
	//     no upstream server is available.
	//   - [access.InternalError] - for internal failures or index conversion errors.
	ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte, criteria optimistic_sync.Criteria) ([]byte, *accessmodel.ExecutorMetadata, error)
}

type TransactionsAPI interface {
	SendTransaction(ctx context.Context, tx *flow.TransactionBody) error

	GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error)
	GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error)

	GetTransactionResult(ctx context.Context, txID flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
	GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
	GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) ([]*accessmodel.TransactionResult, error)

	GetSystemTransaction(ctx context.Context, txID flow.Identifier, blockID flow.Identifier) (*flow.TransactionBody, error)
	GetSystemTransactionResult(ctx context.Context, txID flow.Identifier, blockID flow.Identifier, encodingVersion entities.EventEncodingVersion) (*accessmodel.TransactionResult, error)
}

type TransactionStreamAPI interface {
	// SubscribeTransactionStatuses subscribes to transaction status updates for a given transaction ID. Monitoring starts
	// from the latest block to obtain the current transaction status. If the transaction is already in the final state
	// ([flow.TransactionStatusSealed] or [flow.TransactionStatusExpired]), all statuses will be prepared and sent to the client
	// sequentially. If the transaction is not in the final state, the subscription will stream status updates until the transaction
	// reaches the final state. Once a final state is reached, the subscription will automatically terminate.
	//
	// If the transaction cannot be sent, the subscription will fail and return a failed subscription.
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
	// If the transaction cannot be sent, the subscription will fail and return a failed subscription.
	SendAndSubscribeTransactionStatuses(
		ctx context.Context,
		tx *flow.TransactionBody,
		requiredEventEncodingVersion entities.EventEncodingVersion,
	) subscription.Subscription
}

// API provides all public-facing functionality of the Flow Access API.
//
// CAUTION: SIMPLIFIED ERROR HANDLING
//   - All endpoints must only return an access.accessSentinel error or nil.
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients, in case of an error, all other return values should be discarded.
type API interface {
	AccountsAPI
	EventsAPI
	ScriptsAPI
	TransactionsAPI
	TransactionStreamAPI

	// Ping responds to requests when the server is up.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.ServiceUnavailable] - if the configured static collection node does not respond to ping.
	Ping(ctx context.Context) error

	// GetNetworkParameters returns the network parameters for the current network.
	GetNetworkParameters(ctx context.Context) accessmodel.NetworkParameters

	// GetNodeVersionInfo returns node version information such as semver, commit, sporkID, protocolVersion, etc
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	GetNodeVersionInfo(ctx context.Context) (*accessmodel.NodeVersionInfo, error)

	// GetLatestBlockHeader returns the latest block header in the chain.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error)

	// GetBlockHeaderByHeight returns the block header at the given height.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no header with the given height was found.
	GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error)

	// GetBlockHeaderByID returns the block header with the given ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no header with the given ID was found.
	GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error)

	// GetLatestBlock returns the latest block in the chain.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error)

	// GetBlockByHeight returns the block at the given height.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no block with the given height was found.
	GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error)

	// GetBlockByID returns the block with the given ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no block with the given ID was found.
	GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error)

	// GetCollectionByID returns a light collection by its ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.ServiceUnavailable] - if the configured upstream access client failed to respond.
	//   - [access.DataNotFoundError] if the collection is not found.
	GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error)

	// GetFullCollectionByID returns a full collection by its ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	// As documented in the [access.API], which we partially implement with this function
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - if the collection is not found.
	GetFullCollectionByID(ctx context.Context, id flow.Identifier) (*flow.Collection, error)

	// GetLatestProtocolStateSnapshot returns the latest finalized snapshot.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error)

	// GetProtocolStateSnapshotByBlockID returns serializable Snapshot for a block, by blockID.
	// The requested block must be finalized, otherwise an error is returned.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no block with the given ID was found.
	//   - [access.InvalidRequestError] - block ID is for an orphaned block and will never have a valid snapshot.
	//   - [access.PreconditionFailedError] - a block was found, but it is not finalized and is above the finalized height.
	GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error)

	// GetProtocolStateSnapshotByHeight returns serializable Snapshot by block height.
	// The block must be finalized (otherwise the by-height query is ambiguous).
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no finalized block with the given height was found.
	GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error)

	// GetExecutionResultForBlockID gets an execution result by its block ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no execution result with the given block ID was found.
	GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error)

	// GetExecutionResultByID gets an execution result by its ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected sentinel errors providing details to clients about failed requests:
	//   - [access.DataNotFoundError] - no execution result with the given ID was found.
	GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error)

	// SubscribeBlocksFromStartBlockID subscribes to the finalized or sealed blocks starting at the
	// requested start block id, up until the latest available block. Once the latest is reached,
	// the stream will remain open and responses are sent for each new block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlocksFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlocksFromStartHeight subscribes to the finalized or sealed blocks starting at the requested
	// start block height, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlocksFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlocksFromLatest subscribes to the finalized or sealed blocks starting at the latest sealed block,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlocksFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockHeadersFromStartBlockID streams finalized or sealed block headers starting at the requested
	// start block id, up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockHeadersFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockHeadersFromStartHeight streams finalized or sealed block headers starting at the requested
	// start block height, up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockHeadersFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockHeadersFromLatest streams finalized or sealed block headers starting at the latest sealed block,
	// up until the latest available block header. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block header as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockHeadersFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockDigestsFromStartBlockID streams finalized or sealed lightweight block starting at the requested
	// start block id, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockDigestsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockDigestsFromStartHeight streams finalized or sealed lightweight block starting at the requested
	// start block height, up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockDigestsFromStartHeight(ctx context.Context, startHeight uint64, blockStatus flow.BlockStatus) subscription.Subscription

	// SubscribeBlockDigestsFromLatest streams finalized or sealed lightweight block starting at the latest sealed block,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Each block is filtered by the provided block status, and only blocks that match blockStatus
	// are returned. blockStatus must be BlockStatusSealed or BlockStatusFinalized.
	//
	// If invalid parameters are supplied, a failed subscription is returned.
	SubscribeBlockDigestsFromLatest(ctx context.Context, blockStatus flow.BlockStatus) subscription.Subscription
}
