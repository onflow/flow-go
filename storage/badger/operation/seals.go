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

func IndexSealPayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexSeal, height, blockID, parentID), sealIDs)
}

func LookupSealPayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, sealIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexSeal, height, blockID, parentID), sealIDs)
}

func VerifySealPayload(height uint64, blockID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexSeal, height), makePrefix(codeIndexSeal, uint64(0)), verifypayload(blockID, sealIDs))
}
