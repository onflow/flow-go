package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// InsertProtocolKVStore inserts a protocol KV store by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
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

// IndexProtocolKVStore indexes a protocol KV store by block ID. The function is idempotent, i.e. it accepts
// repeated calls with the same pairs of (blockID, protocolKVStoreID).
//
// CAUTION: To prevent data corruption, we need to guarantee atomicity of existence-check and the subsequent
// database write. Hence, we require the caller to acquire [storage.LockInsertBlock] and hold it until the
// database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a _different_ KV store for the given stateID has already been persisted
func IndexProtocolKVStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, protocolKVStoreID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	key := MakePrefix(codeProtocolKVStoreByBlockID, blockID)
	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		// no-op if the *same* stateID is already indexed for the blockID
		if existing == protocolKVStoreID {
			return nil
		}

		return fmt.Errorf("for block id %x, a kv-store snapshot (%v) is already persisted which is different than the given one (%v): %w",
			blockID[:],
			existing, protocolKVStoreID,
			storage.ErrDataMismatch)
	}
	if !errors.Is(err, storage.ErrNotFound) { // `storage.ErrNotFound` is expected, as this indicates that no receipt is indexed yet; anything else is an exception
		return fmt.Errorf("could not check if kv-store snapshot with block id (%x) exists: %w", blockID[:], irrecoverable.NewException(err))
	}

	return UpsertByKey(rw.Writer(), key, protocolKVStoreID)
}

// LookupProtocolKVStore finds protocol KV store ID by block ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if the given ID does not correspond to any known block
func LookupProtocolKVStore(r storage.Reader, blockID flow.Identifier, protocolKVStoreID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStoreByBlockID, blockID), protocolKVStoreID)
}
