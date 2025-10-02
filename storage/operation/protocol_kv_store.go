package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// InsertProtocolKVStore inserts a protocol KV store by protocol kv store ID.
// Error returns:
//   - [storage.ErrAlreadyExists] if the key already exists in the database.
func InsertProtocolKVStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	key := MakePrefix(codeProtocolKVStore, protocolKVStoreID)
	exists, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return fmt.Errorf("could not check if kv-store snapshot %x exists: %w", protocolKVStoreID[:], irrecoverable.NewException(err))
	}
	if exists {
		return fmt.Errorf("a kv-store snapshot with id %x already exists: %w", protocolKVStoreID[:], storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, kvStore)
}

// RetrieveProtocolKVStore retrieves a protocol KV store by ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no protocol KV with the given ID store exists
func RetrieveProtocolKVStore(r storage.Reader, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// IndexProtocolKVStore indexes a protocol KV store by block ID.
//
// CAUTION: To prevent data corruption, we need to guarantee atomicity of existence-check and the subsequent
// database write. Hence, we require the caller to acquire [storage.LockInsertBlock] and hold it until the
// database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrAlreadyExists] if a KV store for the given blockID has already been indexed
func IndexProtocolKVStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, protocolKVStoreID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	key := MakePrefix(codeProtocolKVStoreByBlockID, blockID)
	exists, err := KeyExists(rw.GlobalReader(), key)
	if err != nil {
		return fmt.Errorf("could not check if kv-store snapshot with block id (%x) exists: %w", blockID[:], irrecoverable.NewException(err))
	}
	if exists {
		return fmt.Errorf("a kv-store snapshot for block id (%x) already exists: %w", blockID[:], storage.ErrAlreadyExists)
	}

	return UpsertByKey(rw.Writer(), key, protocolKVStoreID)
}

// LookupProtocolKVStore finds protocol KV store ID by block ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the given ID does not correspond to any known block
func LookupProtocolKVStore(r storage.Reader, blockID flow.Identifier, protocolKVStoreID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}
