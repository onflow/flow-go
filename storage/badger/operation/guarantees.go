package operation

import (
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

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

// FindDescendants uses guarantee payload index to find all the incorporated descendants for a given block.
// Useful for finding all the unfinalized and incorporated blocks
// "Incorporated" means a block connects to a given block through a certain number of intermediate blocks
// Note: the descendants doesn't include the blockID itself
func FindDescendants(height uint64, blockID flow.Identifier, descendants *[]flow.Identifier) func(*badger.Txn) error {
	// height + 1 to exclude the blockID, and all blocks at the same height
	return iterate(makePrefix(codeIndexGuarantee, height+1), makePrefix(codeIndexGuarantee, uint64(math.MaxUint64)), keyonly(finddescendant(blockID, descendants)))
}
