// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertCommit(blockID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeCommit, blockID), commit)
}

func RetrieveCommit(blockID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCommit, blockID), commit)
}

func IndexCommit(finalID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCommit, finalID), commit)
}

func LookupCommit(finalID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCommit, finalID), commit)
}
