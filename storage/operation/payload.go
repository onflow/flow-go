package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

func InsertSeal(w storage.Writer, sealID flow.Identifier, seal *flow.Seal) error {
	return UpsertByKey(w, MakePrefix(codeSeal, sealID), seal)
}

func RetrieveSeal(r storage.Reader, sealID flow.Identifier, seal *flow.Seal) error {
	return RetrieveByKey(r, MakePrefix(codeSeal, sealID), seal)
}

func IndexPayloadSeals(w storage.Writer, blockID flow.Identifier, sealIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

func LookupPayloadSeals(r storage.Reader, blockID flow.Identifier, sealIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadSeals, blockID), sealIDs)
}

func IndexPayloadReceipts(w storage.Writer, blockID flow.Identifier, receiptIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func IndexPayloadResults(w storage.Writer, blockID flow.Identifier, resultIDs []flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codePayloadResults, blockID), resultIDs)
}

func IndexPayloadProtocolStateID(w storage.Writer, blockID flow.Identifier, stateID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadProtocolStateID(r storage.Reader, blockID flow.Identifier, stateID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadProtocolStateID, blockID), stateID)
}

func LookupPayloadReceipts(r storage.Reader, blockID flow.Identifier, receiptIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadReceipts, blockID), receiptIDs)
}

func LookupPayloadResults(r storage.Reader, blockID flow.Identifier, resultIDs *[]flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codePayloadResults, blockID), resultIDs)
}

// IndexLatestSealAtBlock persists the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func IndexLatestSealAtBlock(w storage.Writer, blockID flow.Identifier, sealID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToLatestSealID, blockID), sealID)
}

// LookupLatestSealAtBlock finds the highest seal that was included in the fork up to (and including) blockID.
// In most cases, it is the highest seal included in this block's payload. However, if there are no
// seals in this block, sealID should reference the highest seal in blockID's ancestor.
func LookupLatestSealAtBlock(r storage.Reader, blockID flow.Identifier, sealID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToLatestSealID, blockID), &sealID)
}

// IndexFinalizedSealByBlockID indexes the _finalized_ seal by the sealed block ID.
// Example: A <- B <- C(SealA)
// when block C is finalized, we create the index `A.ID->SealA.ID`
func IndexFinalizedSealByBlockID(w storage.Writer, sealedBlockID flow.Identifier, sealID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), sealID)
}

// LookupBySealedBlockID finds the seal for the given sealed block ID.
func LookupBySealedBlockID(r storage.Reader, sealedBlockID flow.Identifier, sealID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeBlockIDToFinalizedSeal, sealedBlockID), &sealID)
}

func InsertExecutionForkEvidence(w storage.Writer, conflictingSeals []*flow.IncorporatedResultSeal) error {
	return UpsertByKey(w, MakePrefix(codeExecutionFork), conflictingSeals)
}

func RemoveExecutionForkEvidence(w storage.Writer) error {
	return RemoveByKey(w, MakePrefix(codeExecutionFork))
}

func RetrieveExecutionForkEvidence(r storage.Reader, conflictingSeals *[]*flow.IncorporatedResultSeal) error {
	return RetrieveByKey(r, MakePrefix(codeExecutionFork), conflictingSeals)
}
