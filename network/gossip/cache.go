package gossip

import (
	"sync"
)

// hashCache implements an on-memory cache of recent hashes received and confirmed
// this is for the gossip layer to have a memory of the digests of messages that it received, and
// to help it discard a duplicate message at the proposal time, i.e., the time that another node proposes a new
// message to the node by just sending its hash. By comparing the hash proposal against the hashes in the cache, the node
// can make sure that it is very likely to receive a new message instead of a duplicate, and hence to save its bandwidth
// and resources. The cache implementation may be replaced by a service provided by an upper layer.
type hashCache interface {
	receive(hash string)
	isReceived(hash string) bool
	confirm(hash string)
	isConfirmed(hash string) bool
}

// memoryHashCache implements an on-memory cache of hashes.
type memoryHashCache struct {
	rcv  memorySet
	cnfm memorySet
}

// newMemoryHashCache returns an empty cache
func newMemoryHashCache() *memoryHashCache {
	return &memoryHashCache{
		rcv:  newMemorySet(),
		cnfm: newMemorySet(),
	}
}

// recieve takes a hash and sets it as received in the memory hash cache
func (mhc *memoryHashCache) receive(hash string) {
	mhc.rcv.put(hash)
}

// isRecieved takes a hash and checks if this hash is received
func (mhc *memoryHashCache) isReceived(hash string) bool {
	return mhc.rcv.contains(hash)
}

// confirm takes a hash and sets it as confirmed
// hash: the hash to be checked
func (mhc *memoryHashCache) confirm(hash string) {
	mhc.cnfm.put(hash)
}

// isConfirmed takes a hash and checks if this hash is confirmed
// hash: the hash to be checked
func (mhc *memoryHashCache) isConfirmed(hash string) bool {
	return mhc.cnfm.contains(hash)
}

//A basic set which supports putting keys and checking if keys exist
type set interface {
	put(key string)
	contains(key string) bool
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

// put adds the key to the cache
// key: the key of interest to be added to the cache
func (m *memorySet) put(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mp[key] = true
}

// contains checks if a key exists in the cache
// key: the key of interest to be checked in the cache
func (m *memorySet) contains(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.mp[key]; ok {
		return true
	}
	return false
}
