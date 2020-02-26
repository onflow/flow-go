// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertCommit inserts a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func InsertCommit(blockID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeCommit, blockID), commit)
}

// RetrieveCommit gets a state commitment.
//
// State commitments are keyed by the block whose execution results in the state with the given commit.
func RetrieveCommit(blockID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}

// IndexCommit indexes a state commitment.
//
// State commitments are indexed here by the block that seals the execution state with the given commit.
func IndexCommit(finalID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCommit, finalID), commit)
}

// LookupCommit gets a state commitment.
//
// State commitments are indexed here by the block that seals the execution state with the given commit.
func LookupCommit(finalID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCommit, finalID), commit)
}
