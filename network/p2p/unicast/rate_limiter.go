package unicast

import (
	"sync"
	"time"

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

// GetTimeNow callback used to get the current time. This allows us to improve testing by manipulating the current time
// as opposed to using time.Now directly.
type GetTimeNow func() time.Time

// rateLimitedPeers stores an entry into the underlying map for a peer indicating the peer is currently rate limited.
type rateLimitedPeers struct {
	mu    sync.RWMutex
	peers map[peer.ID]struct{}
}

func newRateLimitedPeers() *rateLimitedPeers {
	return &rateLimitedPeers{
		mu:    sync.RWMutex{},
		peers: make(map[peer.ID]struct{}),
	}
}

// get returns ok if peerID key exists in map
func (r *rateLimitedPeers) exists(peerID peer.ID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.peers[peerID]
	return ok
}

// store stores peeerID as key in underlying map denoting this peer as rate limited.
func (r *rateLimitedPeers) store(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peerID] = struct{}{}
}

// remove deletes peeerID key from underlying map denoting this peer as not rate limited.
func (r *rateLimitedPeers) remove(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, peerID)
}

// limiters stores a rate.Limiter for each peer in an underlying map.
type limiters struct {
	mu   sync.RWMutex
	lmap map[peer.ID]*rate.Limiter
}

func newLimiters() *limiters {
	return &limiters{
		mu:   sync.RWMutex{},
		lmap: make(map[peer.ID]*rate.Limiter),
	}
}

// get returns limiter in limiters map
func (l *limiters) get(peerID peer.ID) (*rate.Limiter, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	limiter, ok := l.lmap[peerID]
	return limiter, ok
}

// store stores limiter in limiters map
func (l *limiters) store(peerID peer.ID, limiter *rate.Limiter) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lmap[peerID] = limiter
}
