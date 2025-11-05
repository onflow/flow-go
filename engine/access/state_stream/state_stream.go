package state_stream

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

const (
	// DefaultRegisterIDsRequestLimit defines the default limit of register IDs for a single request to the get register endpoint
	DefaultRegisterIDsRequestLimit = 100
)

type ExecutionDataAPI interface {
	// GetExecutionDataByBlockID retrieves execution data for a specific block by its block ID.
	//
	// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
	//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
	//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
	//
	// Expected errors:
	// - [access.DataNotFoundError]: when data required to process the request is not available.
	GetExecutionDataByBlockID(
		ctx context.Context,
		blockID flow.Identifier,
		criteria optimistic_sync.Criteria,
	) (*execution_data.BlockExecutionData, *accessmodel.ExecutorMetadata, error)

	// SubscribeExecutionData is deprecated and will be removed in future versions.
	// Use SubscribeExecutionDataFromStartBlockID, SubscribeExecutionDataFromStartBlockHeight or SubscribeExecutionDataFromLatest.
	//
	// SubscribeExecutionData subscribes to execution data starting from a specific block ID and block height.
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) subscription.Subscription

	// SubscribeExecutionDataFromStartBlockID subscribes to execution data starting from a specific block id.
	SubscribeExecutionDataFromStartBlockID(ctx context.Context, startBlockID flow.Identifier) subscription.Subscription

	// SubscribeExecutionDataFromStartBlockHeight subscribes to execution data starting from a specific block height.
	SubscribeExecutionDataFromStartBlockHeight(ctx context.Context, startBlockHeight uint64) subscription.Subscription

	// SubscribeExecutionDataFromLatest subscribes to execution data starting from latest block.
	SubscribeExecutionDataFromLatest(ctx context.Context) subscription.Subscription
}

type EventsAPI interface {
	// SubscribeEvents is deprecated and will be removed in a future version.
	// Use SubscribeEventsFromStartBlockID, SubscribeEventsFromStartHeight or SubscribeEventsFromLatest.
	//
	// SubscribeEvents streams events for all blocks starting at the specified block ID or block height
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Only one of startBlockID and startHeight may be set. If neither startBlockID nor startHeight is provided,
	// the latest sealed block is used.
	//
	// Events within each block are filtered by the provided EventFilter, and only
	// those events that match the filter are returned. If no filter is provided,
	// all events are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
	// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
	// - filter: The event filter used to filter events.
	//
	// If invalid parameters will be supplied SubscribeEvents will return a failed subscription.
	SubscribeEvents(
		ctx context.Context,
		startBlockID flow.Identifier,
		startHeight uint64,
		filter EventFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription

	// SubscribeEventsFromStartBlockID streams events starting at the specified block ID,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Events within each block are filtered by the provided EventFilter, and only
	// those events that match the filter are returned. If no filter is provided,
	// all events are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block.
	// - filter: The event filter used to filter events.
	//
	// If invalid parameters will be supplied SubscribeEventsFromStartBlockID will return a failed subscription.
	SubscribeEventsFromStartBlockID(
		ctx context.Context,
		startBlockID flow.Identifier,
		filter EventFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription

	// SubscribeEventsFromStartHeight streams events starting at the specified block height,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Events within each block are filtered by the provided EventFilter, and only
	// those events that match the filter are returned. If no filter is provided,
	// all events are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startHeight: The height of the starting block.
	// - filter: The event filter used to filter events.
	//
	// If invalid parameters will be supplied SubscribeEventsFromStartHeight will return a failed subscription.
	SubscribeEventsFromStartHeight(
		ctx context.Context,
		startHeight uint64,
		filter EventFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription

	// SubscribeEventsFromLatest subscribes to events starting at the latest sealed block,
	// up until the latest available block. Once the latest is
	// reached, the stream will remain open and responses are sent for each new
	// block as it becomes available.
	//
	// Events within each block are filtered by the provided EventFilter, and only
	// those events that match the filter are returned. If no filter is provided,
	// all events are returned.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - filter: The event filter used to filter events.
	//
	// If invalid parameters will be supplied SubscribeEventsFromLatest will return a failed subscription.
	SubscribeEventsFromLatest(
		ctx context.Context,
		filter EventFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription
}

type AccountsAPI interface {
	// SubscribeAccountStatusesFromStartBlockID subscribes to the streaming of account status changes starting from
	// a specific block ID with an optional status filter.
	SubscribeAccountStatusesFromStartBlockID(
		ctx context.Context,
		startBlockID flow.Identifier,
		filter AccountStatusFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription

	// SubscribeAccountStatusesFromStartHeight subscribes to the streaming of account status changes starting from
	// a specific block height, with an optional status filter.

	SubscribeAccountStatusesFromStartHeight(
		ctx context.Context,
		startHeight uint64,
		filter AccountStatusFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription

	// SubscribeAccountStatusesFromLatestBlock subscribes to the streaming of account status changes starting from a
	// latest sealed block, with an optional status filter.
	SubscribeAccountStatusesFromLatestBlock(
		ctx context.Context,
		filter AccountStatusFilter,
		criteria optimistic_sync.Criteria,
	) subscription.Subscription
}

// API represents an interface that defines methods for interacting with a blockchain's execution data and events.
type API interface {
	AccountsAPI
	EventsAPI
	ExecutionDataAPI

	// GetRegisterValues returns register values for a set of register IDs at the provided block height.
	GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
}
