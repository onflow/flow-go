package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// NOTE: the following functions have the same functionality as
// the corresponding BadgerDB-specific implementations in
// badger/operation/payload.go

// HasExecutionForkEvidence checks if conflicting seals record exists in the database.
// No errors are expected during normal operations.
func HasExecutionForkEvidence(r storage.Reader) (bool, error) {
	return KeyExists(r, MakePrefix(codeExecutionFork))
}

// RetrieveExecutionForkEvidence reads conflicting seals from the database.
// It returns `storage.ErrNotFound` error if no database record is present.
func RetrieveExecutionForkEvidence(r storage.Reader, conflictingSeals *[]*flow.IncorporatedResultSeal) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionFork), conflictingSeals)
}

// RemoveExecutionForkEvidence deletes conflicting seals record from the database.
// No errors are expected during normal operations.
func RemoveExecutionForkEvidence(w storage.Writer) error {
	return RemoveByKey(w, MakePrefix(codeExecutionFork))
}

// InsertExecutionForkEvidence upserts conflicting seals to the database.
// If a record already exists, it is NOT overwritten, the new record is ignored.
// The caller must hold the [storage.LockInsertExecutionForkEvidence] lock.
// No errors are expected during normal operations.
func InsertExecutionForkEvidence(lctx lockctx.Proof, rw storage.ReaderBatchWriter, conflictingSeals []*flow.IncorporatedResultSeal) error {
	if !lctx.HoldsLock(storage.LockInsertExecutionForkEvidence) {
		return fmt.Errorf("InsertExecutionForkEvidence requires LockInsertBlock to be held")
	}
	key := MakePrefix(codeExecutionFork)
	exist, err := KeyExists(rw.GlobalReader(), MakePrefix(codeExecutionFork))
	if err != nil {
		return fmt.Errorf("failed to check if execution fork evidence exists: %w", err)
	}

	if exist {
		// Some evidence about execution fork already stored;
		// We only keep the first evidence => nothing more to do
		return nil
	}

	return UpsertByKey(rw.Writer(), key, conflictingSeals)
}
