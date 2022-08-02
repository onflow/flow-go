package unicast

import (
	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/network/message"
)

// StreamsRateLimiterImpl unicast rate limiter that limits the amount of streams that can
// be created per some configured interval. A new stream is created each time a libP2P
// node sends a direct message.
type StreamsRateLimiterImpl struct {
	*rateLimitedPeers
	*limiters
	limit rate.Limit
	burst int
}

// NewStreamsRateLimiter returns a new StreamsRateLimiterImpl.
func NewStreamsRateLimiter(limit rate.Limit, burst int) *StreamsRateLimiterImpl {
	return &StreamsRateLimiterImpl{
		rateLimitedPeers: new(rateLimitedPeers),
		limiters:         new(limiters),
		limit:            limit,
		burst:            burst,
	}
}

// Allow checks the cached limiter for the peer and returns limiter.Allow().
// If a limiter is not cached for a one is created.
func (s *StreamsRateLimiterImpl) Allow(peerID peer.ID, _ *message.Message) bool {
	limiter := s.getOrStoreLimiter(peerID.String(), rate.NewLimiter(s.limit, s.burst))
	if !limiter.Allow() {
		s.storeRateLimitedPeer(peerID.String())
		return false
	} else {
		s.removeRateLimitedPeer(peerID.String())
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (s *StreamsRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	return s.isRateLimited(peerID.String())
}
