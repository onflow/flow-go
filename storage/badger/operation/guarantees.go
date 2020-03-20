package operation

import (
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// can't use MaxUint64:
// because constant 18446744073709551615 overflows int
const MAX_BLOCK_HEIGHT = uint64(math.MaxUint32)

func InsertGuarantee(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guarantee.CollectionID), guarantee)
}

func CheckGuarantee(collID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeGuarantee, collID), exists)
}

func RetrieveGuarantee(collID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, collID), guarantee)
}

func IndexGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, guaranteeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), guaranteeIDs)
}

func LookupGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, collIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), collIDs)
}

// VerifyGuaranteePayload verifies that the candidate collection IDs
// don't exist in any ancestor block.
func VerifyGuaranteePayload(height uint64, blockID flow.Identifier, collIDs []flow.Identifier) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, uint64(0)), validatepayload(blockID, collIDs))
}

// CheckGuaranteePayload populates `invalidIDs` with any IDs in the candidate
// set that already exist in an ancestor block.
func CheckGuaranteePayload(height uint64, blockID flow.Identifier, candidateIDs []flow.Identifier, invalidIDs *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, uint64(0)), searchduplicates(blockID, candidateIDs, invalidIDs))
}

// FindDecendants finds all the incorporated decendants for a given block.
// Useful for finding all the incorporated unfinalized blocks for the finalized block.
// "Incorporated" means blocks have to all connected to the given block.
func FindDecendants(height uint64, blockID flow.Identifier, decendants *[]flow.Identifier) func(*badger.Txn) error {
	return iterateKey(makePrefix(codeIndexGuarantee, height), makePrefix(codeIndexGuarantee, MAX_BLOCK_HEIGHT), finddescendant(blockID, decendants))
}
