package gnode

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

//messageDatabase is an on memory database interface that keeps a copy of the messages that a node
//receives. This is done as a support to the hash proposals, i.e., to avoid duplicate messages, nodes may
//send the hash first, and seek a confirmation from the receiver on the entire content of the message.
//Although the interface may remain the same, but the implementation is subject to change especially if a database
//is provided by the application layer.
type messageDatabase interface {
	Get(string) (*shared.GossipMessage, error)
	Put(string, *shared.GossipMessage) error
}

type memoryMsgDatabase struct {
	mu    sync.Mutex
	store map[string]*shared.GossipMessage
}

// newMemMsgDatabase returns an empty database
func newMemMsgDatabase() *memoryMsgDatabase {
	return &memoryMsgDatabase{
		store: make(map[string]*shared.GossipMessage),
	}
}

// Put adds a message to the database with its hash as a key
func (mdb *memoryMsgDatabase) Put(hash string, msg *shared.GossipMessage) error {
	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	mdb.store[hash] = msg
	return nil
}

// Get returns a GossipMessage corresponding to the given hash
func (mdb *memoryMsgDatabase) Get(hash string) (*shared.GossipMessage, error) {
	mdb.mu.Lock()
	defer mdb.mu.Unlock()

	msg, exists := mdb.store[hash]

	if !exists {
		return nil, fmt.Errorf("no message found for given hash")
	}

	return msg, nil
}
