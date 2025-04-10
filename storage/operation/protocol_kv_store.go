package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertProtocolKVStore inserts a protocol KV store by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertProtocolKVStore(w storage.Writer, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	return UpsertByKey(w, MakePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// RetrieveProtocolKVStore retrieves a protocol KV store by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveProtocolKVStore(r storage.Reader, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// IndexProtocolKVStore indexes a protocol KV store by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer
func IndexProtocolKVStore(w storage.Writer, blockID flow.Identifier, protocolKVStoreID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}

// LookupProtocolKVStore finds protocol KV store ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupProtocolKVStore(r storage.Reader, blockID flow.Identifier, protocolKVStoreID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}
