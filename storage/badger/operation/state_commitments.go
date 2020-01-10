// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func PersistStateCommitment(id flow.Identifier, commitment *flow.StateCommitment) func(*badger.Txn) error {
	return persist(makePrefix(codeHashToStateCommitment, id), commitment)
}

func RetrieveStateCommitment(id flow.Identifier, commitment *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHashToStateCommitment, id), commitment)
}
