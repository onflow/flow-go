package streamer

import (
	"time"
)

const (
	// DefaultSendBufferSize is the default buffer size for the subscription's send channel.
	// The size is chosen to balance memory overhead from each subscription with performance when
	// streaming existing data.
	DefaultSendBufferSize = 10

	// DefaultSendTimeout is the default timeout for sending a message to the client. After the timeout
	// expires, the connection is closed.
	DefaultSendTimeout = 30 * time.Second

	// DefaultResponseLimit is default max responses per second allowed on a stream. After exceeding
	// the limit, the stream is paused until more capacity is available.
	DefaultResponseLimit = 0

	// DefaultHeartbeatInterval specifies the block interval at which heartbeat messages should be sent.
	DefaultHeartbeatInterval = 1
)

type StreamOptions struct {
	SendTimeout    time.Duration
	SendBufferSize int
	Heartbeat      time.Duration

	// ResponseLimit is the max responses per second allowed on a stream. After exceeding the limit,
	// the stream is paused until more capacity is available. Searches of past data can be CPU
	// intensive, so this helps manage the impact.
	ResponseLimit int
}

func NewDefaultStreamOptions() StreamOptions {
	return StreamOptions{
		SendTimeout:    DefaultSendTimeout,
		SendBufferSize: DefaultSendBufferSize,
		Heartbeat:      DefaultHeartbeatInterval,
		ResponseLimit:  DefaultResponseLimit,
	}
}
