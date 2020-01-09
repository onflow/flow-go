// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func PersistStateCommitment(hash crypto.Hash, commitment *flow.StateCommitment) func(*badger.Txn) error {
	return persist(makePrefix(codeHashToStateCommitment, hash), commitment)
}

func RetrieveStateCommitment(hash crypto.Hash, commitment *flow.StateCommitment) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHashToStateCommitment, hash), commitment)
}
