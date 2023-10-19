package state_stream

import (
	"context"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) Subscription
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
}

// Subscription represents a streaming request, and handles the communication between the grpc handler
// and the backend implementation.
type Subscription interface {
	// ID returns the unique identifier for this subscription used for logging
	ID() string

	// Channel returns the channel from which subscription data can be read
	Channel() <-chan interface{}

	// Err returns the error that caused the subscription to fail
	Err() error
}
