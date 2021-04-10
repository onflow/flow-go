package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
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

func IndexPayloadReceipts(blockID flow.Identifier, receiptIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func IndexPayloadResults(blockID flow.Identifier, resultIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadResults, blockID), resultIDs)
}

func LookupPayloadReceipts(blockID flow.Identifier, receiptIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func LookupPayloadResults(blockID flow.Identifier, resultIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadResults, blockID), resultIDs)
}

func IndexBlockSeal(blockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockToSeal, blockID), sealID)
}

func LookupBlockSeal(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockToSeal, blockID), &sealID)
}

func InsertExecutionForkEvidence(conflictingSeals []*flow.IncorporatedResultSeal) func(*badger.Txn) error {
	return insert(makePrefix(codeExecutionFork), conflictingSeals)
}

func RemoveExecutionForkEvidence() func(*badger.Txn) error {
	return remove(makePrefix(codeExecutionFork))
}

func RetrieveExecutionForkEvidence(conflictingSeals *[]*flow.IncorporatedResultSeal) func(*badger.Txn) error {
	return retrieve(makePrefix(codeExecutionFork), conflictingSeals)
}
