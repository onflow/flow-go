package operation

import (
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
// If a record already exists, it is overwritten; otherwise a new record is created.
// No errors are expected during normal operations.
func InsertExecutionForkEvidence(w storage.Writer, conflictingSeals []*flow.IncorporatedResultSeal) error {
	return UpsertByKey(w, MakePrefix(codeExecutionFork), conflictingSeals)
}
