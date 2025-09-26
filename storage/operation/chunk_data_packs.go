package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertChunkDataPack inserts a [storage.StoredChunkDataPack] into the database, keyed by its chunk ID.
// The function ensures data integrity by first checking if a chunk data pack already exists for the given
// chunk ID and rejecting overwrites with different values. This function is idempotent, i.e. repeated calls
// with the *initially* stored value are no-ops.
//
// CAUTION:
//   - Confirming that no value is already stored and the subsequent write must be atomic to prevent data corruption.
//     The caller must acquire the [storage.LockInsertChunkDataPack] and hold it until the database write has been committed.
//
// Expected error returns during normal operations:
//   - [storage.ErrDataMismatch] if a *different* chunk data pack is already stored for the same chunk ID
func InsertChunkDataPack(lctx lockctx.Proof, rw storage.ReaderBatchWriter, c *storage.StoredChunkDataPack) error {
	if !lctx.HoldsLock(storage.LockInsertChunkDataPack) {
		return fmt.Errorf("InsertChunkDataPack requires lock: %s", storage.LockInsertChunkDataPack)
	}

	key := MakePrefix(codeChunkDataPack, c.ChunkID)

	var existing storage.StoredChunkDataPack
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		err := c.Equals(existing)
		if err != nil {
			return fmt.Errorf("attempting to store conflicting chunk data pack (chunk ID: %v): storing: %+v, stored: %+v, err: %s. %w",
				c.ChunkID, c, &existing, err, storage.ErrDataMismatch)
		}
		return nil // already stored, nothing to do
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("checking for existing chunk data pack (chunk ID: %v): %w", c.ChunkID, err)
	}

	return UpsertByKey(rw.Writer(), key, c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by chunk ID.
// it returns storage.ErrNotFound if the chunk data pack is not found
func RetrieveChunkDataPack(r storage.Reader, chunkID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return RetrieveByKey(r, MakePrefix(codeChunkDataPack, chunkID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given chunk ID.
// any error are exceptions
func RemoveChunkDataPack(w storage.Writer, chunkID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeChunkDataPack, chunkID))
}
