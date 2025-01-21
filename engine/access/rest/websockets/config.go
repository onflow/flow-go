package websockets

import (
	"time"
)

const (
	// PingPeriod defines the interval at which ping messages are sent to the client.
	// This value must be less than pongWait, cause it that case the server ensures it sends a ping well before the PongWait
	// timeout elapses. Each new pong message resets the server's read deadline, keeping the connection alive as long as
	// the client is responsive.
	//
	// Example:
	// At t=9, the server sends a ping, initial read deadline is t=10 (for the first message)
	// At t=10, the client responds with a pong. The server resets its read deadline to t=20.
	// At t=18, the server sends another ping. If the client responds with a pong at t=19, the read deadline is extended to t=29.
	//
	// In case of failure:
	// If the client stops responding, the server will send a ping at t=9 but won't receive a pong by t=10. The server then closes the connection.
	PingPeriod = (PongWait * 9) / 10

	// PongWait specifies the maximum time to wait for a pong response message from the peer
	// after sending a ping
	PongWait = 10 * time.Second

	// WriteWait specifies a timeout for the write operation. If the write
	// isn't completed within this duration, it fails with a timeout error.
	// SetWriteDeadline ensures the write operation does not block indefinitely
	// if the client is slow or unresponsive. This prevents resource exhaustion
	// and allows the server to gracefully handle timeouts for delayed writes.
	WriteWait = 10 * time.Second

	// DefaultMaxSubscriptionsPerConnection defines the default maximum number
	// of WebSocket subscriptions allowed per connection.
	DefaultMaxSubscriptionsPerConnection = 10

	// DefaultMaxResponsesPerSecond defines the default maximum number of responses
	// that can be sent to a single client per second.
	DefaultMaxResponsesPerSecond = 5

	// DefaultInactivityTimeout is the default duration a WebSocket connection can remain open without any active subscriptions
	// before being automatically closed
	DefaultInactivityTimeout time.Duration = 1 * time.Minute
)

type Config struct {
	// MaxSubscriptionsPerConnection specifies the maximum number of active
	// WebSocket subscriptions allowed per connection. If a client attempts
	// to create more subscriptions than this limit, an error will be returned,
	// and the additional subscriptions will be rejected.
	MaxSubscriptionsPerConnection uint64
	// MaxResponsesPerSecond defines the maximum number of responses that
	// can be sent to a single client per second.
	MaxResponsesPerSecond uint64
	// InactivityTimeout specifies the duration a WebSocket connection can remain open without any active subscriptions
	// before being automatically closed
	InactivityTimeout time.Duration
}

func NewDefaultWebsocketConfig() Config {
	return Config{
		MaxSubscriptionsPerConnection: DefaultMaxSubscriptionsPerConnection,
		MaxResponsesPerSecond:         DefaultMaxResponsesPerSecond,
		InactivityTimeout:             DefaultInactivityTimeout,
	}
}
