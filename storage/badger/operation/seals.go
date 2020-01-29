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

func IndexSeal(payloadHash flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexSeal, payloadHash, sealID), sealID)
}

func LookupSeals(payloadHash flow.Identifier, sealIDs *[]flow.Identifier) func(*badger.Txn) error {
	return traverse(makePrefix(codeIndexSeal, payloadHash), lookup(sealIDs))
}
