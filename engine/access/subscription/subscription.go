package subscription

import (
	"context"
	"time"
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
)

// TODO: I'm not sure subscription should be an interface. The impl will unlikely change.
// I guess it is because lots of code depends on it. We don't want to depend on impl details.

type Subscription[T any] interface {
	ID() string
	Channel() <-chan T
	Err() error

	Send(ctx context.Context, value T, timeout time.Duration) error
	CloseWithError(error)
	Close()
}
