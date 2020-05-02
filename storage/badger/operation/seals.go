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

func IndexSealPayload(blockID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexSeal, blockID), sealIDs)
}

func LookupSealPayload(blockID flow.Identifier, sealIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexSeal, blockID), sealIDs)
}
