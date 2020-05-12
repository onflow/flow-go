package mtrie

import (
	"encoding/hex"
	"fmt"
	"sync"
)

// SoftCache keeps about targetSize Memory Tries
type SoftCache struct {
	tries      map[string]*MTrie
	lock       sync.RWMutex
	queue      chan string
	targetSize int
}

// NewSoftCache returns a new cache for memory tries
func NewSoftCache(targetSize int, acceptableDelta int) *SoftCache {
	return &SoftCache{
		tries:      make(map[string]*MTrie),
		lock:       sync.RWMutex{},
		queue:      make(chan string, targetSize+acceptableDelta),
		targetSize: targetSize,
	}
}

// Add adds a new trie to the cache
func (c *SoftCache) Add(mt *MTrie) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.tries[string(mt.rootHash)] = mt
	c.queue <- string(mt.rootHash)
}

// Get returns a trie from the cache by rootHash
func (c *SoftCache) Get(rootHash []byte) (*MTrie, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	trie, ok := c.tries[string(rootHash)]
	if !ok {
		return nil, fmt.Errorf("trie with root hash [%v] not found", hex.EncodeToString(rootHash))
	}
	return trie, nil
}

// Purge clean up the oldest entries of the cache
func (c *SoftCache) Purge(path string) error {
	for len(c.queue) < c.targetSize {
		rootHash := <-c.queue
		trie, ok := c.tries[rootHash]
		if !ok {
			return fmt.Errorf("trie with rootHash [%v] not found", rootHash)
		}
		err := trie.Store(path)
		if err != nil {
			// put the rootHash back
			c.queue <- rootHash
			return fmt.Errorf("failed to persist trie with rootHash [%v]", rootHash)
		}
	}
	return nil
}
