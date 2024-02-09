package badger

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ProtocolKVStore struct {
	db *badger.DB
}

var _ storage.ProtocolKVStore = (*ProtocolKVStore)(nil)

func NewProtocolKVStore(collector module.CacheMetrics) *ProtocolKVStore {
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
