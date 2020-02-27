package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertSeal(seal *flow.Seal) func(*badger.Txn) error {
	return insert(makePrefix(codeSeal, seal.ID()), seal)
}

func CheckSeal(sealID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeSeal, sealID), exists)
}

func RetrieveSeal(sealID flow.Identifier, seal *flow.Seal) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSeal, sealID), seal)
}

func IndexSeal(payloadHash flow.Identifier, index uint64, sealID flow.Identifier) func(*badger.Txn) error {
	//BadgerDB iterates the keys in order - https://github.com/dgraph-io/badger/blob/master/iterator.go#L710
	//Hence adding index at the end should make us retrieve them in order
	return insert(makePrefix(codeIndexSeal, payloadHash, index), sealID)
}

func LookupSeals(payloadHash flow.Identifier, sealIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeIndexSeal, payloadHash), lookup(sealIDs))
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
