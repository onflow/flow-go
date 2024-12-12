package websockets

import (
	"time"
)

const DefaultInactivityTimeout time.Duration = 1 * time.Minute

type Config struct {
	MaxSubscriptionsPerConnection uint64
	MaxResponsesPerSecond         uint64
	SendMessageTimeout            time.Duration
	MaxRequestSize                int64
	// InactivityTimeout specifies the duration a WebSocket connection can remain open without any active subscriptions
	// before being automatically closed
	InactivityTimeout time.Duration
}

func NewDefaultWebsocketConfig() Config {
	return Config{
		MaxSubscriptionsPerConnection: 1000,
		MaxResponsesPerSecond:         1000,
		SendMessageTimeout:            10 * time.Second,
		MaxRequestSize:                1024,
		InactivityTimeout:             DefaultInactivityTimeout,
	}
}
