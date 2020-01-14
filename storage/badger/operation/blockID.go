// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertBlockID(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockID, number), blockID)
}

func PersistBlockID(number uint64, blockID flow.Identifier) func(*badger.Txn) error {
	return persist(makePrefix(codeBlockID, number), blockID)
}

func RetrieveBlockID(number uint64, blockID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockID, number), blockID)
}
