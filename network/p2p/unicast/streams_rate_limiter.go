package unicast

import (
	"fmt"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
	"golang.org/x/time/rate"
)

// StreamsRateLimiterImpl unicast rate limiter that limits the amount of streams that can
// be created per some configured interval. A new stream is created each time a libP2P
// node sends a direct message.
type StreamsRateLimiterImpl struct {
	rateLimitedPeers *rateLimitedPeers
	limiters         *limiters
	limit            rate.Limit
	burst            int
	now              GetTimeNow
}

// NewStreamsRateLimiter returns a new StreamsRateLimiterImpl.
func NewStreamsRateLimiter(limit rate.Limit, burst int, now GetTimeNow) *StreamsRateLimiterImpl {
	return &StreamsRateLimiterImpl{
		rateLimitedPeers: newRateLimitedPeers(),
		limiters:         newLimiters(),
		limit:            limit,
		burst:            burst,
		now:              now,
	}
}

// Allow checks the cached limiter for the peer and returns limiter.Allow().
// If a limiter is not cached for a one is created.
func (s *StreamsRateLimiterImpl) Allow(peerID peer.ID, _ *message.Message) bool {
	limiter := s.getLimiter(peerID)
	if !limiter.AllowN(s.now(), 1) {
		s.rateLimitedPeers.store(peerID)
		return false
	} else {
		fmt.Println("ALLOWED")
		s.rateLimitedPeers.remove(peerID)
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (s *StreamsRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	return s.rateLimitedPeers.exists(peerID)
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (s *StreamsRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if limiter, ok := s.limiters.get(peerID); ok {
		return limiter
	}

	limiter := rate.NewLimiter(s.limit, s.burst)
	s.limiters.store(peerID, limiter)

	return limiter
}
