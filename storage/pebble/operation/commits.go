package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// IndexStateCommitment indexes a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func IndexStateCommitment(blockID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeCommit, blockID), commit)
}

// BatchIndexStateCommitment indexes a state commitment into a batch
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func BatchIndexStateCommitment(blockID flow.Identifier, commit flow.StateCommitment) func(batch *badger.WriteBatch) error {
	return batchWrite(makePrefix(codeCommit, blockID), commit)
}

// LookupStateCommitment gets a state commitment keyed by block ID
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func LookupStateCommitment(blockID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}

// RemoveStateCommitment removes the state commitment by block ID
func RemoveStateCommitment(blockID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeCommit, blockID))
}

// BatchRemoveStateCommitment batch removes the state commitment by block ID
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func BatchRemoveStateCommitment(blockID flow.Identifier) func(batch *badger.WriteBatch) error {
	return batchRemove(makePrefix(codeCommit, blockID))
}
