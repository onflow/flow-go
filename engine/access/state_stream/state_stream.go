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
	// SubscribeEvents subscribes to events starting from a specific block ID and block height, with an optional event filter.
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) subscription.Subscription
	// GetRegisterValues returns register values for a set of register IDs at the provided block height.
	GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
	// SubscribeAccountStatuses subscribes to the streaming of account status changes starting from a specific block ID
	// and block height, with an optional status filter.
	SubscribeAccountStatuses(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
}
