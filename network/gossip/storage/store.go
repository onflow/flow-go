package storage

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/protobuf/gossip/messages"
)

// MemMsgStore implements a thread safe on memory storage
type MemMsgStore struct {
	mu    sync.RWMutex
	store map[string]messages.GossipMessage
}

// NewMemMsgStore returns an empty data base
func NewMemMsgStore() *MemMsgStore {
	return &MemMsgStore{
		store: make(map[string]messages.GossipMessage),
	}
}

// Put adds a message to the storage with its hash as the key
// hash: hash of the message to be stored in the storage
// msg : the message to be stored in the storage
func (mdb *MemMsgStore) Put(hash string, msg *messages.GossipMessage) error {
	mdb.mu.Lock()
	defer mdb.mu.Unlock()

	mdb.store[hash] = *msg

	return nil
}

// Get returns a GossipMessage corresponding to the given hash
// hash: the hash whose message is requested.
func (mdb *MemMsgStore) Get(hash string) (*messages.GossipMessage, error) {
	mdb.mu.RLock()
	defer mdb.mu.RUnlock()

	msg, exists := mdb.store[hash]

	if !exists {
		return nil, fmt.Errorf("no message for the given hash")
	}

	return &msg, nil
}
