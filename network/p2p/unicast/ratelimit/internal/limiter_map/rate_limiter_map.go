package limiter_map

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

type RateLimiterMetadata struct {
	// Limiter the rate Limiter
	Limiter *rate.Limiter
	// LastRateLimit the last timestamp this the peer was rate limited.
	LastRateLimit time.Time
	// LastAccessed the last timestamp when the Limiter was used. This is used to Cleanup old Limiter data
	LastAccessed time.Time
}

// RateLimiterMap stores a RateLimiterMetadata for each peer in an underlying map.
type RateLimiterMap struct {
	mu              sync.Mutex
	ttl             time.Duration
	cleanupInterval time.Duration
	limiters        map[peer.ID]*RateLimiterMetadata
	done            chan struct{}
}

func NewLimiterMap(ttl, cleanupInterval time.Duration) *RateLimiterMap {
	return &RateLimiterMap{
		mu:              sync.Mutex{},
		limiters:        make(map[peer.ID]*RateLimiterMetadata),
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
		done:            make(chan struct{}),
	}
}

// Get returns Limiter in RateLimiterMap map
func (r *RateLimiterMap) Get(peerID peer.ID) (*RateLimiterMetadata, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lmtr, ok := r.limiters[peerID]; ok {
		lmtr.LastAccessed = time.Now()
		return lmtr, ok
	}
	return nil, false
}

// Store stores Limiter in RateLimiterMap map
func (r *RateLimiterMap) Store(peerID peer.ID, lmtr *rate.Limiter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID] = &RateLimiterMetadata{
		Limiter:       lmtr,
		LastAccessed:  time.Now(),
		LastRateLimit: time.Time{},
	}
}

// UpdateLastRateLimit sets the LastRateLimit field of the RateLimiterMetadata for a peer.
func (r *RateLimiterMap) UpdateLastRateLimit(peerID peer.ID, lastRateLimit time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID].LastRateLimit = lastRateLimit
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
		if time.Since(item.LastAccessed) > r.ttl {
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
