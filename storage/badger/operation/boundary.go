// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertBoundary(number uint64) func(*badger.Txn) error {
	return insert(makePrefix(codeBoundary), number)
}

func UpdateBoundary(number uint64) func(*badger.Txn) error {
	return update(makePrefix(codeBoundary), number)
}

func RetrieveBoundary(number *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBoundary), number)
}

// IndexSealIDByBlock indexes latest known sealID by the block. This allows to retrieve a highest sealID
// for every finalized block in a chain
func IndexSealIDByBlock(blockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexSealByBlock, blockID), sealID)
}

// LookupSealIDByBlock retrieves sealID by block for which it was the highest seal.
func LookupSealIDByBlock(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexSealByBlock, blockID), sealID)
}
