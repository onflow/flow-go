package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertProtocolKVStore inserts a protocol KV store by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertProtocolKVStore(protocolKVStoreID flow.Identifier, protocolState *storage.KeyValueStoreData) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolKVStore, protocolKVStoreID), protocolState)
}

// RetrieveProtocolKVStore retrieves a protocol KV store by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveProtocolKVStore(protocolKVStoreID flow.Identifier, kvStore *storage.KeyValueStoreData) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// IndexProtocolKVStore indexes a protocol KV store by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func IndexProtocolKVStore(blockID flow.Identifier, protocolKVStoreID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}

// LookupProtocolKVStore finds protocol KV store ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupProtocolKVStore(blockID flow.Identifier, protocolKVStoreID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}
