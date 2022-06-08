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

// IndexBlockSeal persists the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func IndexBlockSeal(blockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockToSeal, blockID), sealID)
}

// LookupBlockSeal finds the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func LookupBlockSeal(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockToSeal, blockID), &sealID)
}

// IndexBySealedBlockID index a seal by the sealed block ID.
func IndexBySealedBlockID(sealID flow.Identifier, sealedBlockID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeSealedBlockIDIndex, sealedBlockID), sealID)
}

// LookupBySealedBlockID finds the seal for the given sealed block ID.
func LookupBySealedBlockID(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeSealedBlockIDIndex, blockID), &sealID)
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
