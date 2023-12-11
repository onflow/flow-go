package state_stream

import (
	"context"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

const (
	// DefaultSendBufferSize is the default buffer size for the subscription's send channel.
	// The size is chosen to balance memory overhead from each subscription with performance when
	// streaming existing data.
	DefaultSendBufferSize = 10

	// DefaultMaxGlobalStreams defines the default max number of streams that can be open at the same time.
	DefaultMaxGlobalStreams = 1000

	// DefaultCacheSize defines the default max number of objects for the execution data cache.
	DefaultCacheSize = 100

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second

	// DefaultResponseLimit is default max responses per second allowed on a stream. After exceeding
	// the limit, the stream is paused until more capacity is available.
	DefaultResponseLimit = float64(0)

	// DefaultHeartbeatInterval specifies the block interval at which heartbeat messages should be sent.
	DefaultHeartbeatInterval = 1

	// DefaultRegisterIDsRequestLimit defines the default limit of register IDs for a single request to the get register endpoint
	DefaultRegisterIDsRequestLimit = 100
)

// API represents an interface that defines methods for interacting with a blockchain's execution data and events.
type API interface {
	// GetExecutionDataByBlockID retrieves execution data for a specific block by its block ID.
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionData, error)
	// SubscribeExecutionData subscribes to execution data starting from a specific block ID and block height.
	SubscribeExecutionData(ctx context.Context, startBlockID flow.Identifier, startBlockHeight uint64) Subscription
	// SubscribeEvents subscribes to events starting from a specific block ID and block height, with an optional event filter.
	SubscribeEvents(ctx context.Context, startBlockID flow.Identifier, startHeight uint64, filter EventFilter) Subscription
	// GetRegisterValues returns register values for a set of register IDs at the provided block height.
	GetRegisterValues(registerIDs flow.RegisterIDs, height uint64) ([]flow.RegisterValue, error)
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

// Streamable represents a subscription that can be streamed.
type Streamable interface {
	ID() string
	Close()
	Fail(error)
	Send(context.Context, interface{}, time.Duration) error
	Next(context.Context) (interface{}, error)
}
