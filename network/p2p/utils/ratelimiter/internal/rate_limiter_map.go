package internal

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/module/irrecoverable"
)

type rateLimiterMetadata struct {
	mu *sync.RWMutex
	// limiter the rate limiter
	limiter *rate.Limiter
	// lastRateLimit the last timestamp this the peer was rate limited.
	lastRateLimit time.Time
	// lastAccessed the last timestamp when the limiter was used. This is used to Cleanup old limiter data
	lastAccessed time.Time
}

// newRateLimiterMetadata returns a new rateLimiterMetadata
func newRateLimiterMetadata(limiter *rate.Limiter) *rateLimiterMetadata {
	return &rateLimiterMetadata{
		mu:            &sync.RWMutex{},
		limiter:       limiter,
		lastAccessed:  time.Now(),
		lastRateLimit: time.Time{},
	}
}

// Limiter returns rateLimiterMetadata.limiter..
func (m *rateLimiterMetadata) Limiter() *rate.Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiter
}

// LastRateLimit returns rateLimiterMetadata.lastRateLimit.
func (m *rateLimiterMetadata) LastRateLimit() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRateLimit
}

// SetLastRateLimit sets rateLimiterMetadata.lastRateLimit.
func (m *rateLimiterMetadata) SetLastRateLimit(lastRateLimit time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastRateLimit = lastRateLimit
}

// LastAccessed returns rateLimiterMetadata.lastAccessed.
func (m *rateLimiterMetadata) LastAccessed() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastAccessed
}

// SetLastAccessed sets rateLimiterMetadata.lastAccessed.
func (m *rateLimiterMetadata) SetLastAccessed(lastAccessed time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastAccessed = lastAccessed
}

// RateLimiterMap stores a rateLimiterMetadata for each peer in an underlying map.
type RateLimiterMap struct {
	// mu read write mutex used to synchronize updates to the rate limiter map.
	mu sync.RWMutex
	// ttl time to live is the duration in which a rate limiter is stored in the limiters map.
	// Stale rate limiters from peers that have not interacted in a while will be cleaned up to
	// free up unused resources.
	ttl time.Duration
	// cleanupInterval the interval in which stale rate limiter's are removed from the limiters map
	// to free up unused resources.
	cleanupInterval time.Duration
	// limiters map that stores rate limiter metadata for each peer.
	limiters map[peer.ID]*rateLimiterMetadata
}

func NewLimiterMap(ttl, cleanupInterval time.Duration) *RateLimiterMap {
	return &RateLimiterMap{
		mu:              sync.RWMutex{},
		limiters:        make(map[peer.ID]*rateLimiterMetadata),
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
	}
}

// Get returns limiter in RateLimiterMap map
func (r *RateLimiterMap) Get(peerID peer.ID) (*rateLimiterMetadata, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if lmtr, ok := r.limiters[peerID]; ok {
		lmtr.SetLastAccessed(time.Now())
		return lmtr, ok
	}
	return nil, false
}

// Store stores limiter in RateLimiterMap map
func (r *RateLimiterMap) Store(peerID peer.ID, lmtr *rate.Limiter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID] = newRateLimiterMetadata(lmtr)
}

// UpdateLastRateLimit sets the lastRateLimit field of the rateLimiterMetadata for a peer.
func (r *RateLimiterMap) UpdateLastRateLimit(peerID peer.ID, lastRateLimit time.Time) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.limiters[peerID].SetLastRateLimit(lastRateLimit)
}

// Remove deletes peerID key from underlying map.
func (r *RateLimiterMap) Remove(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeUnlocked(peerID)
}

// removeUnlocked removes peerID key from underlying map without acquiring a lock.
func (r *RateLimiterMap) removeUnlocked(peerID peer.ID) {
	delete(r.limiters, peerID)
}

// cleanup check the TTL for all keys in map and Remove isExpired keys.
func (r *RateLimiterMap) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for peerID, item := range r.limiters {
		if time.Since(item.LastAccessed()) > r.ttl {
			r.removeUnlocked(peerID)
		}
	}
}

// CleanupLoop starts a loop that periodically removes stale peers.
// This func blocks until the signaler context is canceled, when context
// is canceled the limiter map is cleaned up before the cleanup loop exits.
func (r *RateLimiterMap) CleanupLoop(ctx irrecoverable.SignalerContext) {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	defer r.cleanup()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanup()
		}
	}
}
