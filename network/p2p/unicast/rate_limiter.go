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

// peer unicast rate limiter => contains streams/sec limiter and bandwidth/sec limiter
//
// 	stream/sec - peerID => Limiter
// 				 use rate.Allow() to check for unicast req/sec (the equivalent of rate limiting streams per sec)
//
//  bandwidth/sec - peerID => Limiter
//					use rate.AllowN(msg_size) to check if reading message is within rate limit
