// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"time"
)

// Config represents the configuration of the base network layer.
type Config struct {
	MinConns          uint          `validate:"min=1" env:"NET_MIN_CONNS,default=8"`
	MaxConns          uint          `validate:"min=1,gtefield=MinPeers" env:"NET_MAX_CONNS,default=16"`
	HeartbeatInterval time.Duration `validate:"-" env:"NET_HEARTBEAT_INTERVAL"`
	ShuffleInterval   time.Duration `validate:"-" env:"NET_SHUFFLE_INTERVAL"`
	ListenAddress     string        `validate:"omitempty,uri" env:"NET_LISTEN_ADDRESS"`
}
