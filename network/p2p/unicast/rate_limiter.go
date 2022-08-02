package unicast

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/network/message"
)

// RateLimiter unicast rate limiter interface
type RateLimiter interface {
	// Allow returns true if a message should be allowed to be processed.
	Allow(peerID peer.ID, msg *message.Message) bool

	// IsRateLimited returns true if a peer is rate limited.
	IsRateLimited(peerID peer.ID) bool
}

// rateLimitedPeers stores an entry into the underlying sync.Map for a peer indicating the peer is currently rate limited.
type rateLimitedPeers struct {
	sync.Map
}

// storeRateLimitedPeer stores peeerID as key in underlying map denoting this peer as rate limited.
func (r *rateLimitedPeers) storeRateLimitedPeer(peerID string) {
	r.Store(peerID, struct{}{})
}

// removeRateLimitedPeer removes peeerID key from underlying map denoting this peer as not rate limited.
func (r *rateLimitedPeers) removeRateLimitedPeer(peerID string) {
	r.Delete(peerID)
}

// isRateLimited returns true if an entry for the peerID provided exists.
func (r *rateLimitedPeers) isRateLimited(peerID string) bool {
	_, ok := r.Load(peerID)
	return ok
}

// limiters stores a rate.Limiter for each peer in an underlying sync.Map.
type limiters struct {
	sync.Map
}

// getLimiter returns limiter for peerID.
func (l *limiters) getOrStoreLimiter(peerID string, limiter *rate.Limiter) *rate.Limiter {
	storedLimiter, _ := l.LoadOrStore(peerID, limiter)
	return storedLimiter.(*rate.Limiter)
}
