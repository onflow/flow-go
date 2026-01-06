package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/storage"
)

var (
	// IncompleteStateError indicates that some information cannot be retrieved from the database,
	// which the protocol mandates to be present. This can be a symptom of a corrupted state
	// or an incorrectly / incompletely bootstrapped node. In most cases, this is an exception.
	//
	// ATTENTION: in most cases, [IncompleteStateError] error is a symptom of a corrupted state
	// or an incorrectly / incompletely bootstrapped node. Typically, this is an unexpected exception
	// and should not be checked for the same way as benign sentinel errors.
	IncompleteStateError = errors.New("data required by protocol is missing in database")
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

// RetrieveFinalizedHeight reads height of the latest finalized block directly from the database.
//
// During bootstrapping, the latest finalized block and its height are indexed and thereafter the
// latest finalized height is only updated (but never removed). Hence, for a properly bootstrapped
// node, this function should _always_ return a proper value.
//
// CAUTION: This function should only be called on properly bootstrapped nodes. If the state is
// corrupted or the node is not properly bootstrapped, this function may return [IncompleteStateError].
// The reason for not returning [storage.ErrNotFound] directly is to avoid confusion between an often
// benign [storage.ErrNotFound] and failed reads of quantities that the protocol mandates to be present.
//
// No error returns are expected during normal operations.
func RetrieveFinalizedHeight(r storage.Reader, height *uint64) error {
	var h uint64
	err := RetrieveByKey(r, MakePrefix(codeFinalizedHeight), &h)
	if err != nil {
		// mask the lower-level error to prevent confusion with the often benign `storage.ErrNotFound`:
		return fmt.Errorf("latest finalized height could not be read, which should never happen for bootstrapped nodes: %w", IncompleteStateError)
	}
	*height = h
	return nil
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

// RetrieveSealedHeight reads height of the latest sealed block directly from the database.
//
// During bootstrapping, the latest sealed block and its height are indexed and thereafter the
// latest sealed height is only updated (but never removed). Hence, for a properly bootstrapped
// node, this function should _always_ return a proper value.
//
// CAUTION: This function should only be called on properly bootstrapped nodes. If the state is
// corrupted or the node is not properly bootstrapped, this function may return [IncompleteStateError].
// The reason for not returning [storage.ErrNotFound] directly is to avoid confusion between an often
// benign [storage.ErrNotFound] and failed reads of quantities that the protocol mandates to be present.
//
// No error returns are expected during normal operations.
func RetrieveSealedHeight(r storage.Reader, height *uint64) error {
	var h uint64
	err := RetrieveByKey(r, MakePrefix(codeSealedHeight), &h)
	if err != nil {
		// mask the lower-level error to prevent confusion with the often benign `storage.ErrNotFound`:
		return fmt.Errorf("latest sealed height could not be read, which should never happen for bootstrapped nodes: %w", IncompleteStateError)
	}
	*height = h
	return nil
}

// InsertEpochFirstHeight inserts the height of the first block in the given epoch.
// The first block of an epoch E is the finalized block with view >= E.FirstView.
// Although we don't store the final height of an epoch, it can be inferred from this index.
// The caller must hold [storage.LockFinalizeBlock]. This function enforces each index is written exactly once.
// Returns [storage.ErrAlreadyExists] if the height has already been indexed.
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
