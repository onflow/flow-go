package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertSeal(sealID flow.Identifier, seal *flow.Seal) func(*badger.Txn) error {
	return insert(makePrefix(codeSeal, sealID), seal)
}

func RetrieveSeal(sealID flow.Identifier, seal *flow.Seal) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSeal, sealID), seal)
}

func IndexPayloadSeals(blockID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadSeals, blockID), sealIDs)
}

func LookupPayloadSeals(blockID flow.Identifier, sealIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadSeals, blockID), sealIDs)
}

func IndexBlockSeal(blockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockToSeal, blockID), sealID)
}

func LookupBlockSeal(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockToSeal, blockID), &sealID)
}
