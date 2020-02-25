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
//
// State commitments are keyed by the block ID of the block whose final state
// is the state being committed to.
func RetrieveCommit(blockID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}

// IndexCommit indexes a state commitment.
//
// State commitments are indexed by the block ID of the block in which the
// commitment is sealed/finalized.
func IndexCommit(finalID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCommit, finalID), commit)
}

// LookupCommit gets a state commitment.
//
// State commitments are indexed by the block ID of the block in which the
// commitment is sealed/finalized.
func LookupCommit(finalID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCommit, finalID), commit)
}
