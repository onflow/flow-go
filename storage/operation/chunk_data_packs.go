package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexChunkDataPackByChunkID inserts a mapping from chunk ID to stored chunk data pack ID. It requires
// the [storage.LockIndexChunkDataPackByChunkID] lock to be acquired by the caller and held until the write batch has been committed.
// Returns [storage.ErrDataMismatch] if a different chunk data pack ID already exists for the given chunk ID.
func IndexChunkDataPackByChunkID(lctx lockctx.Proof, rw storage.ReaderBatchWriter, chunkID flow.Identifier, chunkDataPackID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockIndexChunkDataPackByChunkID) {
		return fmt.Errorf("missing required lock: %s", storage.LockIndexChunkDataPackByChunkID)
	}
	key := MakePrefix(codeIndexChunkDataPackByChunkID, chunkID)
	var existing flow.Identifier
	err := RetrieveByKey(rw.GlobalReader(), key, &existing)
	if err == nil {
		if existing == chunkDataPackID {
			// already exists, nothing to do
			return nil
		}
		return fmt.Errorf("cannot insert chunk data pack ID for chunk %s, different one exist: existing: %v, new: %v: %w",
			chunkID, existing, chunkDataPackID, storage.ErrDataMismatch)
	} else if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("cannot check existing chunk data pack ID for chunk %s: %w", chunkID, err)
	}

	return UpsertByKey(rw.Writer(), key, &chunkDataPackID)
}

// RetrieveChunkDataPackID retrieves the stored chunk data pack ID for a given chunk ID.
// Returns [storage.ErrNotFound] if no chunk data pack has been indexed as result for the given chunk ID.
func RetrieveChunkDataPackID(r storage.Reader, chunkID flow.Identifier, chunkDataPackID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexChunkDataPackByChunkID, chunkID), chunkDataPackID)
}

// RemoveChunkDataPackID removes the mapping from chunk ID to stored chunk data pack ID.
// Non-existing keys are no-ops. Any errors are exceptions.
func RemoveChunkDataPackID(w storage.Writer, chunkID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeIndexChunkDataPackByChunkID, chunkID))
}

// InsertChunkDataPack inserts a [storage.StoredChunkDataPack] into the database, keyed by its own ID.
//
// CAUTION: The caller must ensure `storeChunkDataPackID` is the same as `c.ID()`, ie. a collision-resistant
// hash of the chunk data pack! This method silently overrides existing data, which is safe only if for the
// same key, we always write the same value.
//
// No error returns expected during normal operations.
func InsertChunkDataPack(rw storage.ReaderBatchWriter, storeChunkDataPackID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return UpsertByKey(rw.Writer(), MakePrefix(codeChunkDataPack, storeChunkDataPackID), c)
}

// RetrieveChunkDataPack retrieves a chunk data pack by stored chunk data pack ID.
// It returns [storage.ErrNotFound] if no chunk data pack with the given ID is known.
func RetrieveChunkDataPack(r storage.Reader, storeChunkDataPackID flow.Identifier, c *storage.StoredChunkDataPack) error {
	return RetrieveByKey(r, MakePrefix(codeChunkDataPack, storeChunkDataPackID), c)
}

// RemoveChunkDataPack removes the chunk data pack with the given stored chunk data pack ID.
// Non-existing keys are no-ops. Any errors are exceptions.
func RemoveChunkDataPack(w storage.Writer, chunkDataPackID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeChunkDataPack, chunkDataPackID))
}
