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
func InsertProtocolKVStore(w storage.Writer, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	return UpsertByKey(w, MakePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// RetrieveProtocolKVStore retrieves a protocol KV store by ID.
// Expected error returns during normal operations:
//   - [storage.ErrNotFound] if no protocol KV with the given ID store exists
func RetrieveProtocolKVStore(r storage.Reader, protocolKVStoreID flow.Identifier, kvStore *flow.PSKeyValueStoreData) error {
	return RetrieveByKey(r, MakePrefix(codeProtocolKVStore, protocolKVStoreID), kvStore)
}

// IndexProtocolKVStore indexes a protocol KV store by block ID.
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

		return fmt.Errorf("kv-store snapshot with block id (%x) already exists and different (%v != %v): %w",
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
