package pebble

import (
	"time"
)

const (
	DefaultPruningInterval      = uint64(2_000_000)
	DefaultThreshold            = uint64(100_000)
	DefaultPruningThrottleDelay = 10 * time.Minute
)
