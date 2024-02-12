package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// DefaultProtocolKVStoreCacheSize is the default size for primary protocol KV store cache.
// KV store is rarely updated, so we will have a limited number of unique states.
// Lets be generous and assume we have 10 different KV stores used at the same time.
var DefaultProtocolKVStoreCacheSize uint = 10

// DefaultProtocolKVStoreByBlockIDCacheSize is the default value for secondary byBlockIdCache.
// We want to be able to cover a broad interval of views without cache misses, so we use a bigger value.
var DefaultProtocolKVStoreByBlockIDCacheSize uint = 1000

type ProtocolKVStore struct {
	db *badger.DB
}

var _ storage.ProtocolKVStore = (*ProtocolKVStore)(nil)

func NewProtocolKVStore(collector module.CacheMetrics,
	stateCacheSize uint,
	stateByBlockIDCacheSize uint,
) *ProtocolKVStore {
	return &ProtocolKVStore{}
}

func (s *ProtocolKVStore) StoreTx(stateID flow.Identifier, data *storage.KeyValueStoreData) func(*transaction.Tx) error {
	//TODO implement me
	panic("implement me")
}

func (s *ProtocolKVStore) IndexTx(blockID flow.Identifier, stateID flow.Identifier) func(*transaction.Tx) error {
	//TODO implement me
	panic("implement me")
}

func (s *ProtocolKVStore) ByID(id flow.Identifier) (*storage.KeyValueStoreData, error) {
	//TODO implement me
	panic("implement me")
}

func (s *ProtocolKVStore) ByBlockID(blockID flow.Identifier) (*storage.ProtocolKVStore, error) {
	//TODO implement me
	panic("implement me")
}
