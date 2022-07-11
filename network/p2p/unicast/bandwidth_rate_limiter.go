package unicast

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/message"
	"golang.org/x/time/rate"
)

var (
	defaultBandwidthRateLimitInterval = rate.Every(time.Second) // default time interval in between rate limits
	defaultBandwidthRateLimitBurst    = 1_048_576               // bytes allowed to be sent per interval above
)

// BandWidthRateLimiter unicast rate limiter that limits the bandwidth that can be sent
// by a peer per some configured interval.
type BandWidthRateLimiter struct {
	lock     sync.Mutex
	limiters map[peer.ID]*rate.Limiter
}

// Allow checks the cached limiter for the peer and returns limiter.AllowN(msg.Size())
// which will check if a peer is able to send a message of msg.Size().
// If a limiter is not cached for a one is created.
func (b *BandWidthRateLimiter) Allow(peerID peer.ID, msg *message.Message) bool {
	limiter := b.limiters[peerID]
	if limiter == nil {
		limiter = b.setNewLimiter(peerID)
	}

	return limiter.AllowN(time.Now(), msg.Size())
}

// setNewLimiter creates and caches a new limiter for the provided peer.
func (b *BandWidthRateLimiter) setNewLimiter(peerID peer.ID) *rate.Limiter {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.limiters[peerID] = rate.NewLimiter(defaultBandwidthRateLimitInterval, defaultBandwidthRateLimitBurst)
	return b.limiters[peerID]
}
