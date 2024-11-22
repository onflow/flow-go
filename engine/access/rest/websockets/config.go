package websockets

import (
	"time"
)

type Config struct {
	MaxSubscriptionsPerConnection uint64
	MaxResponsesPerSecond         uint64
	SendMessageTimeout            time.Duration
	MaxRequestSize                int64
}

func NewDefaultWebsocketConfig() Config {
	return Config{
		MaxSubscriptionsPerConnection: 1000,
		MaxResponsesPerSecond:         1000,
		SendMessageTimeout:            10 * time.Second,
		MaxRequestSize:                1024,
	}
}
