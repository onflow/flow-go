package unicast

import (
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

type rateLimiterMapItem struct {
	*rate.Limiter
	lastActive time.Time
}

// rateLimiterMap stores a rate.Limiter for each peer in an underlying map.
type rateLimiterMap struct {
	mu              sync.Mutex
	ttl             time.Duration
	cleanupInterval time.Duration
	limiters        map[peer.ID]*rateLimiterMapItem
	done            chan struct{}
}

func newLimiterMap(ttl, cleanupTick time.Duration) *rateLimiterMap {
	return &rateLimiterMap{
		mu:              sync.Mutex{},
		limiters:        make(map[peer.ID]*rateLimiterMapItem),
		ttl:             ttl,
		cleanupInterval: cleanupTick,
		done:            make(chan struct{}),
	}
}

// get returns limiter in rateLimiterMap map
func (r *rateLimiterMap) get(peerID peer.ID) (*rate.Limiter, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if lmtr, ok := r.limiters[peerID]; ok {
		lmtr.lastActive = time.Now()
		return lmtr.Limiter, ok
	}
	return nil, false
}

// store stores limiter in rateLimiterMap map
func (r *rateLimiterMap) store(peerID peer.ID, lmtr *rate.Limiter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[peerID] = &rateLimiterMapItem{lmtr, time.Now()}
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
func (r *rateLimiterMap) isExpired(item *rateLimiterMapItem) bool {
	return time.Since(item.lastActive) > r.ttl
}
