package middleware

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// ConnectionCache is a cache of all the WriteConnections established so far
type ConnectionCache struct {
	sync.Mutex
	conns map[flow.Identifier]*WriteConnection
}

// NewConnectionCache creates a new connection cache
func NewConnectionCache() *ConnectionCache {
	return &ConnectionCache{conns: make(map[flow.Identifier]*WriteConnection)}
}

// Add will add the given conn with the given address to our list in a
// concurrency-safe manner.
func (cc *ConnectionCache) Add(nodeID flow.Identifier, conn *WriteConnection) {
	cc.Lock()
	defer cc.Unlock()
	cc.conns[nodeID] = conn
}

// Exists will check if the connection already exists in a concurrency-safe manner.
func (cc *ConnectionCache) Exists(nodeID flow.Identifier) bool {
	cc.Lock()
	defer cc.Unlock()
	_, ok := cc.conns[nodeID]
	return ok
}

// Remove will remove the connection with the given nodeID from the list in
// a concurrency-safe manner.
func (cc *ConnectionCache) Remove(nodeID flow.Identifier) {
	cc.Lock()
	defer cc.Unlock()
	delete(cc.conns, nodeID)
}

// Get will return the connections from the cache for the give nodeID
func (cc *ConnectionCache) Get(nodeID flow.Identifier) (*WriteConnection, bool) {
	cc.Lock()
	defer cc.Unlock()
	c, found := cc.conns[nodeID]
	return c, found
}

// GetAll will get all connections in the cache
func (cc *ConnectionCache) GetAll() []*WriteConnection {
	cc.Lock()
	defer cc.Unlock()
	allc := make([]*WriteConnection, 0, len(cc.conns))

	for _, value := range cc.conns {
		allc = append(allc, value)
	}
	return allc
}
