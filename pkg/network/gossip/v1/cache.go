package gnode

import "sync"

// cache implements an on-,memory cache of recent hashes received and confirmed
//this is for the gossip layer to have a memory of the digests of messages that it received, and
//to help it discard a duplicate message at the proposal time, i.e., the time that another node proposes a new
//message to the node by just sending its hash. By comparing the hash proposal against the hashes in the cache, the node
//can make sure that it is very likely to receive a new message instead of a duplicate, and hence to save its bandwidth
//and resources. The cache implementation may be replaced by a service provided by an upper layer.
//Todo: a cache size management
type hashCache interface {
	receive(hash string)
	isReceived(hash string) bool
	confirm(hash string)
	isConfirmed(hash string) bool
}

type memoryHashCache struct {
	receivedCache  memorySet
	confirmedCache memorySet
}

// newMemoryHashCache returns an empty cache, it accommodate a two-level cache, one for the messages that the
// node has entirely received, and the other one for the proposals that the node received and confirmed and
// waits for the entire message to be sent.
func newMemoryHashCache() *memoryHashCache {
	return &memoryHashCache{
		receivedCache:  newMemorySet(),
		confirmedCache: newMemorySet(),
	}
}

//receive inserts hash of a completely received message into the receivedCache in order not to receive any duplication
//of this anymore
func (mhc *memoryHashCache) receive(hash string) {
	mhc.receivedCache.put(hash)
}

//isReceived checks the existence of the input hash in the received messages cache
func (mhc *memoryHashCache) isReceived(hash string) bool {
	return mhc.receivedCache.contains(hash)
}

//confirm adds a hash proposal into the confirmedCache, which means that the node received the proposal but not
//the actual message yet
func (mhc *memoryHashCache) confirm(hash string) {
	mhc.confirmedCache.put(hash)
}

//confirm returns true if one have already received a hash proposal with the input hash but not yet the actual message
func (mhc *memoryHashCache) isConfirmed(hash string) bool {
	return mhc.confirmedCache.contains(hash)
}

// memorySet is a thread safe set
type memorySet struct {
	mp map[string]bool
	mu sync.RWMutex
}

// newMemorySet returns an empty set
func newMemorySet() memorySet {
	return memorySet{
		mp: make(map[string]bool),
	}
}

func (m *memorySet) put(key string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.mp[key] = true
}

func (m *memorySet) contains(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.mp[key]; ok {
		return true
	}
	return false
}
