// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertHeader(header *flow.Header) func(*badger.Txn) error {
	return insert(makePrefix(codeHeader, header.Hash()), header)
}

func RetrieveHeader(hash crypto.Hash, header *flow.Header) func(*badger.Txn) error {
	return retrieve(makePrefix(codeHeader, hash), header)
}
