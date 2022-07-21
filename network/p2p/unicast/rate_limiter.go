package unicast

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
)

// RateLimiter unicast rate limiter interface
type RateLimiter interface {
	// Allow returns true if a message should be allowed to be processed
	Allow(peerID peer.ID, msg *message.Message) bool
}
