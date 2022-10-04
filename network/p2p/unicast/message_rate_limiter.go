package unicast

import (
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/network/message"
)

// MessageRateLimiterImpl unicast rate limiter that limits the amount of streams that can
// be created per some configured interval. A new stream is created each time a libP2P
// node sends a direct message.
type MessageRateLimiterImpl struct {
	rateLimitedPeers *rateLimitedPeersMap
	limiters         *rateLimiterMap
	limit            rate.Limit
	burst            int
	now              GetTimeNow
}

// NewMessageRateLimiter returns a new MessageRateLimiterImpl. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewMessageRateLimiter(limit rate.Limit, burst int, opts ...RateLimiterOpt) *MessageRateLimiterImpl {
	l := &MessageRateLimiterImpl{
		rateLimitedPeers: newRateLimitedPeersMap(rateLimiterTTL, cleanUpTickInterval),
		limiters:         newLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:            limit,
		burst:            burst,
		now:              time.Now,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Allow checks the cached limiter for the peer and returns limiter.Allow().
// If a limiter is not cached for a peer one is created.
func (s *MessageRateLimiterImpl) Allow(peerID peer.ID, _ *message.Message) bool {
	limiter := s.getLimiter(peerID)
	if !limiter.AllowN(s.now(), 1) {
		s.rateLimitedPeers.store(peerID)
		return false
	} else {
		s.rateLimitedPeers.remove(peerID)
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (s *MessageRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	return s.rateLimitedPeers.exists(peerID)
}

// Start starts cleanup loop for underlying caches.
func (s *MessageRateLimiterImpl) Start() {
	go s.limiters.cleanupLoop()
	go s.rateLimitedPeers.cleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
// is closed it can not be reused.
func (s *MessageRateLimiterImpl) Stop() {
	s.limiters.close()
	s.rateLimitedPeers.close()
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (s *MessageRateLimiterImpl) SetTimeNowFunc(now GetTimeNow) {
	s.now = now
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (s *MessageRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if limiter, ok := s.limiters.get(peerID); ok {
		return limiter
	}

	limiter := rate.NewLimiter(s.limit, s.burst)
	s.limiters.store(peerID, limiter)

	return limiter
}
