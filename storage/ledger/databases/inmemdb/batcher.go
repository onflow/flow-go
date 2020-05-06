package inmemdb

import (
	"encoding/hex"
	"sync"

	"github.com/dapperlabs/flow-go/storage/ledger/databases"
)

// InMemBatcher is an in memory batcher
type InMemBatcher struct {
	Batch map[string][]byte
	lock  sync.Mutex
}

// Put puts things into batcher
func (mb *InMemBatcher) Put(key, value []byte) {
	mb.lock.Lock()
	mb.Batch[hex.EncodeToString(key)] = value
	mb.lock.Unlock()
}

// Delete deletes things from batcher
func (mb *InMemBatcher) Delete(key []byte) {
	mb.lock.Lock()
	delete(mb.Batch, hex.EncodeToString(key))
	mb.lock.Unlock()
}

func NewInMemBatcher() *InMemBatcher {
	return &InMemBatcher{Batch: make(map[string][]byte)}
}

func (mb *InMemBatcher) Merge(other databases.Batcher) {
	b, _ := other.(*InMemBatcher)
	// TODO
	// if !ok {
	// 	return fmt.Errorf("pruning supported between the same databases only")
	// }

	for key, value := range b.Batch {
		mb.Batch[key] = value
	}
}
