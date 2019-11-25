package cache

import (
	"sync"
)


// MemHashCache implements an on-memory cache of hashes.
// Note: as both underlying fields of memoryHashCache are thread-safe, the memoryHashCache itself does
// not require a mutex lock
type MemHashCache struct {
	rcv  memorySet
	cnfm memorySet
}

// NewMemHashCache returns an empty cache7
func NewMemHashCache() *MemHashCache {
	return &MemHashCache{
		rcv:  newMemorySet(),
		cnfm: newMemorySet(),
	}
}

// Receive takes a hash and sets it as received in the memory hash cache
func (mhc *MemHashCache) Receive(hash string) bool {
	return mhc.rcv.put(hash)
}

// IsReceived takes a hash and checks if this hash is received
func (mhc *MemHashCache) IsReceived(hash string) bool {
	return mhc.rcv.contains(hash)
}

// Confirm takes a hash and sets it as confirmed
// hash: the hash to be checked
func (mhc *MemHashCache) Confirm(hash string) bool {
	return mhc.cnfm.put(hash)
}

// IsConfirmed takes a hash and checks if this hash is confirmed
// hash: the hash to be checked
func (mhc *MemHashCache) IsConfirmed(hash string) bool {
	return mhc.cnfm.contains(hash)
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
func (m *memorySet) put(key string) (duplicate bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, duplicate = m.mp[key]
	m.mp[key] = true
	return
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
