package validation

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/atomic"
)

// ClusterPrefixedTopicsReceived tracker that keeps track of the number of cluster prefixed topics received in a control message.
type ClusterPrefixedTopicsReceived struct {
	lock sync.RWMutex
	// receivedByPeer cluster prefixed control messages received per peer.
	receivedByPeer map[peer.ID]*atomic.Uint64
}

// Inc increments the counter for the peer, if a counter does not exist one is initialized.
func (c *ClusterPrefixedTopicsReceived) Inc(pid peer.ID) {
	c.lock.Lock()
	defer c.lock.Unlock()
	counter, ok := c.receivedByPeer[pid]
	if !ok {
		c.receivedByPeer[pid] = atomic.NewUint64(1)
		return
	}
	counter.Inc()
}

// Load returns the current count for the peer.
func (c *ClusterPrefixedTopicsReceived) Load(pid peer.ID) uint64 {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if counter, ok := c.receivedByPeer[pid]; ok {
		return counter.Load()
	}
	return 0
}

// Reset resets the counter for a peer.
func (c *ClusterPrefixedTopicsReceived) Reset(pid peer.ID) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if counter, ok := c.receivedByPeer[pid]; ok {
		counter.Store(0)
	}
}

// NewClusterPrefixedTopicsReceivedTracker returns a new *ClusterPrefixedTopicsReceived.
func NewClusterPrefixedTopicsReceivedTracker() *ClusterPrefixedTopicsReceived {
	return &ClusterPrefixedTopicsReceived{
		lock:           sync.RWMutex{},
		receivedByPeer: make(map[peer.ID]*atomic.Uint64, 0),
	}
}
