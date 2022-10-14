package ratelimit

import (
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit/internal/limiter_map"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/p2p"

	"github.com/onflow/flow-go/network/message"
)

// MessageRateLimiterImpl unicast rate limiter that limits the amount of streams that can
// be created per some configured interval. A new stream is created each time a libP2P
// node sends a direct message.
type MessageRateLimiterImpl struct {
	limiters                 *limiter_map.RateLimiterMap
	limit                    rate.Limit
	burst                    int
	now                      p2p.GetTimeNow
	rateLimitLockoutDuration time.Duration // the amount of time that has to pass before a peer is allowed to connect
}

// NewMessageRateLimiter returns a new MessageRateLimiterImpl. The cleanup loop will be started in a
// separate goroutine and should be stopped by calling Close.
func NewMessageRateLimiter(limit rate.Limit, burst, lockoutDuration int, opts ...p2p.RateLimiterOpt) *MessageRateLimiterImpl {
	l := &MessageRateLimiterImpl{
		limiters:                 limiter_map.NewLimiterMap(rateLimiterTTL, cleanUpTickInterval),
		limit:                    limit,
		burst:                    burst,
		now:                      time.Now,
		rateLimitLockoutDuration: time.Duration(lockoutDuration) * time.Second,
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
		s.limiters.UpdateLastRateLimit(peerID, s.now())
		return false
	} else {
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (s *MessageRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	metadata, ok := s.limiters.Get(peerID)
	if !ok {
		return false
	}
	return time.Since(metadata.LastRateLimit) < s.rateLimitLockoutDuration
}

// Start starts cleanup loop for underlying caches.
func (s *MessageRateLimiterImpl) Start() {
	go s.limiters.CleanupLoop()
}

// Stop sends cleanup signal to underlying rate limiters and rate limited peers maps. After the rate limiter
// is closed it can not be reused.
func (s *MessageRateLimiterImpl) Stop() {
	s.limiters.Close()
}

// SetTimeNowFunc overrides the default time.Now func with the GetTimeNow func provided.
func (s *MessageRateLimiterImpl) SetTimeNowFunc(now p2p.GetTimeNow) {
	s.now = now
}

// getLimiter returns limiter for the peerID, if a limiter does not exist one is created and stored.
func (s *MessageRateLimiterImpl) getLimiter(peerID peer.ID) *rate.Limiter {
	if metadata, ok := s.limiters.Get(peerID); ok {
		return metadata.Limiter
	}

	limiter := rate.NewLimiter(s.limit, s.burst)
	s.limiters.Store(peerID, limiter)

	return limiter
}
