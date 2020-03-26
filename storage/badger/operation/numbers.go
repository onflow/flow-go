// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertNumber(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeNumber, number), blockID)
}

func RetrieveNumber(number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeNumber, number), blockID)
}
