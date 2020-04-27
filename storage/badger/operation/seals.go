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

// VerifySealPayload verifies that the candidate seal IDs don't exist
// in any ancestor block.
func VerifySealPayload(height uint64, blockID flow.Identifier, sealIDs []flow.Identifier) func(*badger.Txn) error {
	start, end := payloadIterRange(codeIndexSeal, height, 0)
	return iterate(start, end, validatepayload(blockID, sealIDs))
}

// CheckSealPayload populates `invalidIDs` with any IDs in the candidate
// set that already exist in an ancestor block.
func CheckSealPayload(height uint64, blockID flow.Identifier, candidateIDs []flow.Identifier, invalidIDs *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	start, end := payloadIterRange(codeIndexSeal, height, 0)
	return iterate(start, end, searchduplicates(blockID, candidateIDs, invalidIDs))
}
