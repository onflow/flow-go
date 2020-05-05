package inmemdb

import (
	"encoding/hex"
)

// InMemBatcher is an in memory batcher
type InMemBatcher struct {
	Batch map[string][]byte
}

// Put puts things into batcher
func (mb *InMemBatcher) Put(key, value []byte) {
	mb.Batch[hex.EncodeToString(key)] = value
}

// Delete deletes things from batcher
func (mb *InMemBatcher) Delete(key []byte) {
	delete(mb.Batch, hex.EncodeToString(key))
}

func newInMemBatcher() *InMemBatcher {
	return &InMemBatcher{Batch: make(map[string][]byte)}
}
