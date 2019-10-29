package protocols

import (
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
)

// PeerQueue keeps track prioritizing the streams based on the last time they used
// it operates based on an LRU cache and discards the least recently used connection
type PeerQueue struct {
	cache *lru.Cache
}

// NewPeerQueue constructs a new PeerQueue with the specified size
func NewPeerQueue(size int) (*PeerQueue, error) {

	cache, err := lru.New(size)

	if err != nil {
		return nil, errors.Wrap(err, "could not add LRU cache to peerqueue")
	}

	pq := &PeerQueue{cache: cache}

	return pq, nil
}

// remove method removes a cached connection
// The reason we use Remove is to discard a dead connection from cache
func (pq *PeerQueue) remove(address string) {
	pq.cache.Remove(address)
}

// contains checks if an address has a corresponding cached connection or not
func (pq *PeerQueue) contains(address string) bool {
	return pq.cache.Contains(address)
}

// get returns a connection of a specific address if found
func (pq *PeerQueue) get(address string) (clientStream, bool) {

	stream, ok := pq.cache.Get(address)
	if !ok {
		return nil, false
	}

	return stream.(clientStream), true
}

// add caches a new connection corresponding to a client address
func (pq *PeerQueue) add(address string, stream clientStream) {
	pq.cache.Add(address, stream)
}
