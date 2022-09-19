package unicast

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

type rateLimitedPeerMapItem struct {
	lastActive time.Time
}

// rateLimitedPeersMap concurrency safe struct that stores rate limited peers in a map.
// Map keys will expire or be deleted from the map based on the configured TTL.
type rateLimitedPeersMap struct {
	mu              sync.Mutex
	ttl             time.Duration
	cleanupInterval time.Duration
	peers           map[peer.ID]*rateLimitedPeerMapItem
	done            chan struct{}
}

func newRateLimitedPeersMap(ttl, cleanupTick time.Duration) *rateLimitedPeersMap {
	return &rateLimitedPeersMap{
		mu:              sync.Mutex{},
		ttl:             ttl,
		cleanupInterval: cleanupTick,
		peers:           make(map[peer.ID]*rateLimitedPeerMapItem),
		done:            make(chan struct{}),
	}
}

// get returns ok if peerID key exists in map. This func will also update the lastActive
// timestamp for the item refreshing its ttl.
func (r *rateLimitedPeersMap) exists(peerID peer.ID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if item, ok := r.peers[peerID]; ok {
		item.lastActive = time.Now()
		return true
	}
	return false
}

// store stores peeerID as key in underlying map denoting this peer as rate limited.
func (r *rateLimitedPeersMap) store(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peerID] = &rateLimitedPeerMapItem{lastActive: time.Now()}
}

// remove deletes peeerID key from underlying map denoting this peer as not rate limited.
func (r *rateLimitedPeersMap) remove(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeUnlocked(peerID)
}

// removeUnlocked removes peerID key from underlying map without acquiring a lock.
func (r *rateLimitedPeersMap) removeUnlocked(peerID peer.ID) {
	delete(r.peers, peerID)
}

// cleanup check the TTL for all keys in map and remove isExpired keys.
func (r *rateLimitedPeersMap) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for peerID, item := range r.peers {
		if r.isExpired(item) {
			r.removeUnlocked(peerID)
		}
	}
}

// cleanupLoop starts a loop that periodically removes stale peers.
func (r *rateLimitedPeersMap) cleanupLoop() {
	ticker := time.NewTicker(r.cleanupInterval)
	for {
		select {
		case <-ticker.C:
			r.cleanup()
		case _, ok := <-r.done:
			// clean up and return when channel is closed
			if !ok {
				return
			}
		}
	}
}

// close will close the done channel starting the final full cleanup and stopping the cleanup loop.
func (r *rateLimitedPeersMap) close() {
	close(r.done)
}

// isExpired returns true if configured ttl has passed for an item.
func (r *rateLimitedPeersMap) isExpired(item *rateLimitedPeerMapItem) bool {
	return time.Since(item.lastActive) > r.ttl
}
