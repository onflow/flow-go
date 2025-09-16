package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/storage"
)

// UpsertFinalizedHeight upserts the finalized height index, overwriting the current value.
// Updates to this index must strictly increase the finalized height.
// To enforce this, the caller must check the current finalized height while holding [storage.LockFinalizeBlock].
func UpsertFinalizedHeight(lctx lockctx.Proof, w storage.Writer, height uint64) error {
	if !lctx.HoldsLock(storage.LockFinalizeBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockFinalizeBlock)
	}
	return UpsertByKey(w, MakePrefix(codeFinalizedHeight), height)
}

func RetrieveFinalizedHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeFinalizedHeight), height)
}

// UpsertSealedHeight upserts the latest sealed height, OVERWRITING the current value.
// Updates to this index must strictly increase the sealed height.
// To enforce this, the caller must check the current sealed height while holding [storage.LockFinalizeBlock].
func UpsertSealedHeight(lctx lockctx.Proof, w storage.Writer, height uint64) error {
	if !lctx.HoldsLock(storage.LockFinalizeBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockFinalizeBlock)
	}
	return UpsertByKey(w, MakePrefix(codeSealedHeight), height)
}

func RetrieveSealedHeight(r storage.Reader, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeSealedHeight), height)
}

// InsertEpochFirstHeight inserts the height of the first block in the given epoch.
// The first block of an epoch E is the finalized block with view >= E.FirstView.
// Although we don't store the final height of an epoch, it can be inferred from this index.
// The caller must hold [storage.LockFinalizeBlock]. This function enforces each index is written exactly once.
// Returns storage.ErrAlreadyExists if the height has already been indexed.
func InsertEpochFirstHeight(lctx lockctx.Proof, rw storage.ReaderBatchWriter, epoch, height uint64) error {
	if !lctx.HoldsLock(storage.LockFinalizeBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockFinalizeBlock)
	}

	var existingHeight uint64
	err := RetrieveEpochFirstHeight(rw.GlobalReader(), epoch, &existingHeight)
	if err == nil {
		return storage.ErrAlreadyExists
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("failed to check existing epoch first height: %w", err)
	}

	return UpsertByKey(rw.Writer(), MakePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochFirstHeight retrieves the height of the first block in the given epoch.
// This operation does not require any locks, because the first height of an epoch does not change once set.
// Returns [storage.ErrNotFound] if the first block of the epoch has not yet been finalized.
func RetrieveEpochFirstHeight(r storage.Reader, epoch uint64, height *uint64) error {
	return RetrieveByKey(r, MakePrefix(codeEpochFirstHeight, epoch), height)
}

// RetrieveEpochLastHeight retrieves the height of the last block in the given epoch.
// This operation does not require any locks, because the first height of an epoch does not change once set.
// It's a more readable, but equivalent query to RetrieveEpochFirstHeight when interested in the last height of an epoch.
// Returns [storage.ErrNotFound] if the first block of the epoch has not yet been finalized.
func RetrieveEpochLastHeight(r storage.Reader, epoch uint64, height *uint64) error {
	var nextEpochFirstHeight uint64
	if err := RetrieveByKey(r, MakePrefix(codeEpochFirstHeight, epoch+1), &nextEpochFirstHeight); err != nil {
		return err
	}
	*height = nextEpochFirstHeight - 1
	return nil
}
