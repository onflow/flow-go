package state_stream

import (
	"context"

	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

const (
	// DefaultRegisterIDsRequestLimit defines the default limit of register IDs for a single request to the get register endpoint
	DefaultRegisterIDsRequestLimit = 100
)

// API represents an interface that defines methods for interacting with a blockchain's execution data and events.
type API interface {
	// GetExecutionDataByBlockID retrieves execution data for a specific block by its block ID.
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	// SubscribeExecutionData subscribes to execution data starting from a specific block ID and block height.
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) subscription.Subscription
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
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) subscription.Subscription
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
	SubscribeEventsFromStartBlockID(ctx context.Context, startBlockID flow.Identifier, filter EventFilter) subscription.Subscription
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
	SubscribeEventsFromStartHeight(ctx context.Context, startHeight uint64, filter EventFilter) subscription.Subscription
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
	SubscribeEventsFromLatest(ctx context.Context, filter EventFilter) subscription.Subscription
	// GetRegisterValues returns register values for a set of register IDs at the provided block height.
	GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
}
