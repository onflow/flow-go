package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertChunkDataPackID(lctx lockctx.Proof, rw storage.ReaderBatchWriter, chunkID flow.Identifier, storedChunkDataPackID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertOwnReceipt) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertOwnReceipt)
	}
	key := MakePrefix(codeChunkDataPackID, chunkID)
	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		if existing == storedChunkDataPackID {
			// already exists, nothing to do
			return nil
		}
		return fmt.Errorf("cannot insert chunk data pack ID for chunk %s, different one exist: existing: %v, new: %v: %w",
			chunkID, existing, storedChunkDataPackID, storage.ErrDataMismatch)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("cannot check existing chunk data pack ID for chunk %s: %w", chunkID, err)
	}

	return UpsertByKey(rw.Writer(), key, &storedChunkDataPackID)
}

func RetrieveChunkDataPackID(r storage.Reader, chunkID flow.Identifier, storedChunkDataPackID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeChunkDataPackID, chunkID), storedChunkDataPackID)
}

func RemoveChunkDataPackID(w storage.Writer, chunkID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeChunkDataPackID, chunkID))
}

// InsertStoredChunkDataPack inserts a [storage.StoredChunkDataPack] into the database, keyed by its own ID.
// The caller must ensure the storedChunkDataPackID is the same as c.ID().
//
// No error returns during normal operations:
func InsertStoredChunkDataPack(rw storage.ReaderBatchWriter, storeChunkDataPackID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return UpsertByKey(rw.Writer(), MakePrefix(codeStoredChunkDataPack, storeChunkDataPackID), c)
}

// RetrieveStoredChunkDataPack retrieves a chunk data pack by stored chunk data pack ID.
// it returns storage.ErrNotFound if the chunk data pack is not found
func RetrieveStoredChunkDataPack(r storage.Reader, storeChunkDataPackID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return RetrieveByKey(r, MakePrefix(codeStoredChunkDataPack, storeChunkDataPackID), c)
}

// RemoveStoredChunkDataPack removes the chunk data pack with the given stored chunk data pack ID.
// any error are exceptions
func RemoveStoredChunkDataPack(w storage.Writer, storedChunkDataPackID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeStoredChunkDataPack, storedChunkDataPackID))
}
