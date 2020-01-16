// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertHeader(header *flow.Header) func(*badger.Txn) error {
	return insert(makePrefix(codeHeader, header.ID()), header)
}

func PersistHeader(header *flow.Header) func(*badger.Txn) error {
	return persist(makePrefix(codeHeader, header.ID()), header)
}

func RetrieveHeader(blockID flow.Identifier, header *flow.Header) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeader, blockID), header)
}
