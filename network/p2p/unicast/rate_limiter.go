package unicast

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/network/message"
)

const (
	cleanUpTickDuration = 10 * time.Minute
	rateLimiterTTL      = 10 * time.Minute
)

// RateLimiter unicast rate limiter interface
type RateLimiter interface {
	// Allow returns true if a message should be allowed to be processed.
	Allow(peerID peer.ID, msg *message.Message) bool

	// IsRateLimited returns true if a peer is rate limited.
	IsRateLimited(peerID peer.ID) bool

	// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
	// is stopped it can not be reused.
	Stop()

	// Start starts cleanup loop for underlying rate limiters and rate limited peers maps.
	Start()
}

// GetTimeNow callback used to get the current time. This allows us to improve testing by manipulating the current time
// as opposed to using time.Now directly.
type GetTimeNow func() time.Time
