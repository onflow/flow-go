package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IndexStateCommitment indexes a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func IndexStateCommitment(w storage.Writer, blockID flow.Identifier, commit flow.StateCommitment) error {
	return UpsertByKey(w, MakePrefix(codeCommit, blockID), commit)
}

// LookupStateCommitment gets a state commitment keyed by block ID
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func LookupStateCommitment(r storage.Reader, blockID flow.Identifier, commit *flow.StateCommitment) error {
	return RetrieveByKey(r, MakePrefix(codeCommit, blockID), commit)
}

// RemoveStateCommitment removes the state commitment by block ID
func RemoveStateCommitment(w storage.Writer, blockID flow.Identifier) error {
	return RemoveByKey(w, MakePrefix(codeCommit, blockID))
}
