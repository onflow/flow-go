// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func PersistCommit(commitID flow.Identifier, commit flow.StateCommitment) func(*badger.Txn) error {
	return persist(makePrefix(codeHashToStateCommitment, commitID), commit)
}

func RetrieveCommit(commitID flow.Identifier, commit *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHashToStateCommitment, commitID), commit)
}
