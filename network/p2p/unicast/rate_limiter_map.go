package unicast

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

type rateLimiterMetadata struct {
	// limiter the rate limiter
	limiter *rate.Limiter
	// lastAccessed the last timestamp when the limiter was used. This is used to cleanup old limiter data
	lastAccessed time.Time
	// lastRateLimit the last timestamp this the peer was rate limited.
	lastRateLimit time.Time
}

// rateLimiterMap stores a rateLimiterMetadata for each peer in an underlying map.
type rateLimiterMap struct {
	mu              sync.Mutex
	ttl             time.Duration
	cleanupInterval time.Duration
	limiters        map[peer.ID]*rateLimiterMetadata
	done            chan struct{}
}

func newLimiterMap(ttl, cleanupInterval time.Duration) *rateLimiterMap {
	return &rateLimiterMap{
		mu:              sync.Mutex{},
		limiters:        make(map[peer.ID]*rateLimiterMetadata),
		ttl:             ttl,
		cleanupInterval: cleanupInterval,
		done:            make(chan struct{}),
	}
}

// get returns limiter in rateLimiterMap map
func (r *rateLimiterMap) get(peerID peer.ID) (*rateLimiterMetadata, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lmtr, ok := r.limiters[peerID]; ok {
		lmtr.lastAccessed = time.Now()
		return lmtr, ok
	}
	return nil, false
}

// store stores limiter in rateLimiterMap map
func (r *rateLimiterMap) store(peerID peer.ID, lmtr *rate.Limiter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID] = &rateLimiterMetadata{
		limiter:       lmtr,
		lastAccessed:  time.Now(),
		lastRateLimit: time.Time{},
	}
}

// updateLastRateLimit sets the lastRateLimit field of the rateLimiterMetadata for a peer.
func (r *rateLimiterMap) updateLastRateLimit(peerID peer.ID, lastRateLimit time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID].lastRateLimit = lastRateLimit
}

// remove deletes peerID key from underlying map.
func (r *rateLimiterMap) remove(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeUnlocked(peerID)
}

// removeUnlocked removes peerID key from underlying map without acquiring a lock.
func (r *rateLimiterMap) removeUnlocked(peerID peer.ID) {
	delete(r.limiters, peerID)
}

// cleanup check the TTL for all keys in map and remove isExpired keys.
func (r *rateLimiterMap) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for peerID, item := range r.limiters {
		if r.isExpired(item) {
			r.removeUnlocked(peerID)
		}
	}
}

// cleanupLoop starts a loop that periodically removes stale peers.
func (r *rateLimiterMap) cleanupLoop() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case <-r.done:
			return
		}
	}
}

// close will close the done channel starting the final full cleanup and stopping the cleanup loop.
func (r *rateLimiterMap) close() {
	close(r.done)
}

// isExpired returns true if configured ttl has passed for an item.
func (r *rateLimiterMap) isExpired(item *rateLimiterMetadata) bool {
	return time.Since(item.lastAccessed) > r.ttl
}
