// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertCommit inserts a state commitment.
//
// State commitments are keyed by the block ID of the block whose final state
// is the state being committed to.
func InsertCommit(blockID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeCommit, blockID), commit)
}

// RetrieveCommit gets a state commitment.
func RetrieveCommit(blockID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}
