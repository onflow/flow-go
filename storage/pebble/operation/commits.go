package operation

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
)

// IndexStateCommitment indexes a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func IndexStateCommitment(blockID flow.Identifier, commit flow.StateCommitment) func(pebble.Writer) error {
	return insert(makePrefix(codeCommit, blockID), commit)
}

// LookupStateCommitment gets a state commitment keyed by block ID
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func LookupStateCommitment(blockID flow.Identifier, commit *flow.StateCommitment) func(pebble.Reader) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}

// RemoveStateCommitment removes the state commitment by block ID
func RemoveStateCommitment(blockID flow.Identifier) func(pebble.Writer) error {
	return remove(makePrefix(codeCommit, blockID))
}
