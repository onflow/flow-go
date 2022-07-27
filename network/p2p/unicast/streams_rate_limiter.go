package unicast

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
	"golang.org/x/time/rate"
)

// StreamsRateLimiter unicast rate limiter that limits the amount of streams that can
// be created per some configured interval. A new stream is created each time a libP2P
// node sends a direct message.
type StreamsRateLimiter struct {
	lock     sync.Mutex
	limiters map[peer.ID]*rate.Limiter
	limit    rate.Limit
	burst    int
}

// NewStreamsRateLimiter returns a new StreamsRateLimiter.
func NewStreamsRateLimiter(limit rate.Limit, burst int) *StreamsRateLimiter {
	return &StreamsRateLimiter{
		lock:     sync.Mutex{},
		limiters: make(map[peer.ID]*rate.Limiter),
		limit:    limit,
		burst:    burst,
	}
}

// Allow checks the cached limiter for the peer and returns limiter.Allow().
// If a limiter is not cached for a one is created.
func (s *StreamsRateLimiter) Allow(peerID peer.ID, _ *message.Message) bool {
	limiter := s.limiters[peerID]
	if limiter == nil {
		limiter = s.setNewLimiter(peerID)
	}

	return limiter.Allow()
}

// setNewLimiter creates and caches a new limiter for the provided peer.
func (s *StreamsRateLimiter) setNewLimiter(peerID peer.ID) *rate.Limiter {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.limiters[peerID] = rate.NewLimiter(s.limit, s.burst)
	return s.limiters[peerID]
}
