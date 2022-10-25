package limiter_map

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

type RateLimiterMetadata struct {
	mu *sync.RWMutex
	// limiter the rate limiter
	limiter *rate.Limiter
	// lastRateLimit the last timestamp this the peer was rate limited.
	lastRateLimit time.Time
	// lastAccessed the last timestamp when the limiter was used. This is used to Cleanup old limiter data
	lastAccessed time.Time
}

// newRateLimiterMetadata returns a new RateLimiterMetadata
func newRateLimiterMetadata(limiter *rate.Limiter) *RateLimiterMetadata {
	return &RateLimiterMetadata{
		mu:            &sync.RWMutex{},
		limiter:       limiter,
		lastAccessed:  time.Now(),
		lastRateLimit: time.Time{},
	}
}

// Limiter returns RateLimiterMetadata.limiter..
func (m *RateLimiterMetadata) Limiter() *rate.Limiter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.limiter
}

// LastRateLimit returns RateLimiterMetadata.lastRateLimit.
func (m *RateLimiterMetadata) LastRateLimit() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastRateLimit
}

// SetLastRateLimit sets RateLimiterMetadata.lastRateLimit.
func (m *RateLimiterMetadata) SetLastRateLimit(lastRateLimit time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastRateLimit = lastRateLimit
}

// LastAccessed returns RateLimiterMetadata.lastAccessed.
func (m *RateLimiterMetadata) LastAccessed() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastAccessed
}

// SetLastAccessed sets RateLimiterMetadata.lastAccessed.
func (m *RateLimiterMetadata) SetLastAccessed(lastAccessed time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastAccessed = lastAccessed
}

// RateLimiterMap stores a RateLimiterMetadata for each peer in an underlying map.
type RateLimiterMap struct {
	mu              sync.RWMutex
	ttl             time.Duration
	cleanupInterval time.Duration
	limiters        map[peer.ID]*RateLimiterMetadata
	done            chan struct{}
}

func NewLimiterMap(ttl, cleanupInterval time.Duration) *RateLimiterMap {
	return &RateLimiterMap{
		mu:              sync.RWMutex{},
		limiters:        make(map[peer.ID]*RateLimiterMetadata),
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
		done:            make(chan struct{}),
	}
}

// Get returns limiter in RateLimiterMap map
func (r *RateLimiterMap) Get(peerID peer.ID) (*RateLimiterMetadata, bool) {
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

// UpdateLastRateLimit sets the lastRateLimit field of the RateLimiterMetadata for a peer.
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

// Cleanup check the TTL for all keys in map and Remove isExpired keys.
func (r *RateLimiterMap) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for peerID, item := range r.limiters {
		if time.Since(item.LastAccessed()) > r.ttl {
			r.removeUnlocked(peerID)
		}
	}
}

// CleanupLoop starts a loop that periodically removes stale peers.
func (r *RateLimiterMap) CleanupLoop() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.Cleanup()
		case <-r.done:
			return
		}
	}
}

// Close will Close the done channel starting the final full Cleanup and stopping the Cleanup loop.
func (r *RateLimiterMap) Close() {
	close(r.done)
}
