package unicast

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/message"
)

// BandWidthRateLimiterImpl unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiterImpl struct {
	*rateLimitedPeers
	*limiters
	limit rate.Limit
	burst int
}

// NewBandWidthRateLimiter returns a new BandWidthRateLimiterImpl.
func NewBandWidthRateLimiter(limit rate.Limit, burst int) *BandWidthRateLimiterImpl {
	return &BandWidthRateLimiterImpl{
		rateLimitedPeers: new(rateLimitedPeers),
		limiters:         new(limiters),
		limit:            limit,
		burst:            burst,
	}
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached for a one is created.
func (s *BandWidthRateLimiterImpl) Allow(peerID peer.ID, msg *message.Message) bool {
	limiter := s.getOrStoreLimiter(peerID.String(), rate.NewLimiter(s.limit, s.burst))
	if !limiter.AllowN(time.Now(), msg.Size()) {
		s.storeRateLimitedPeer(peerID.String())
		return false
	} else {
		s.removeRateLimitedPeer(peerID.String())
		return true
	}
}

// IsRateLimited returns true is a peer is currently rate limited.
func (s *BandWidthRateLimiterImpl) IsRateLimited(peerID peer.ID) bool {
	return s.isRateLimited(peerID.String())
}
