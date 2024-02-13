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

func IndexPayloadProtocolStateID(blockID flow.Identifier, stateID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadProtocolStateID(blockID flow.Identifier, stateID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadReceipts(blockID flow.Identifier, receiptIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func LookupPayloadResults(blockID flow.Identifier, resultIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexLatestSealAtBlock persists the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func IndexLatestSealAtBlock(blockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockIDToLatestSealID, blockID), sealID)
}

// LookupLatestSealAtBlock finds the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func LookupLatestSealAtBlock(blockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockIDToLatestSealID, blockID), &sealID)
}

// IndexFinalizedSealByBlockID indexes the _finalized_ seal by the sealed block ID.
// Example: A <- B <- C(SealA)
// when block C is finalized, we create the index `A.ID->SealA.ID`
func IndexFinalizedSealByBlockID(sealedBlockID flow.Identifier, sealID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), sealID)
}

// LookupBySealedBlockID finds the seal for the given sealed block ID.
func LookupBySealedBlockID(sealedBlockID flow.Identifier, sealID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), &sealID)
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
