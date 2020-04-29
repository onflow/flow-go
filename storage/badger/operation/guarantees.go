package operation

import (
	"math"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func InsertGuarantee(guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guarantee.CollectionID), guarantee)
}

func CheckGuarantee(guaranteeid flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeGuarantee, guaranteeid), exists)
}

func RetrieveGuarantee(guaranteeid flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, guaranteeid), guarantee)
}

func IndexGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, guaranteeIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), guaranteeIDs)
}

func LookupGuaranteePayload(height uint64, blockID flow.Identifier, parentID flow.Identifier, guaranteeIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(toPayloadIndex(codeIndexGuarantee, height, blockID, parentID), guaranteeIDs)
}

// VerifyGuaranteePayload verifies that the candidate collection IDs
// don't exist in any ancestor block.
func VerifyGuaranteePayload(upper uint64, lower uint64, blockID flow.Identifier, guaranteeIDs []flow.Identifier) func(*badger.Txn) error {
	start, end := payloadIterRange(codeIndexGuarantee, upper, lower)
	return iterate(start, end, validatepayload(blockID, guaranteeIDs))
}

// CheckGuaranteePayload populates `invalidIDs` with any IDs in the candidate
// set that already exist in an ancestor block.
func CheckGuaranteePayload(height uint64, blockID flow.Identifier, candidateIDs []flow.Identifier, invalidIDs *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	// TODO: Currently Hard coded to only checking the last 10 blocks
	limit := uint64(0)
	if height > 10 {
		limit = height - 10
	}
	start, end := payloadIterRange(codeIndexGuarantee, height, limit)
	return iterate(start, end, searchduplicates(blockID, candidateIDs, invalidIDs))
}

// FindDescendants uses guarantee payload index to find all the incorporated descendants for a given block.
// Useful for finding all the un-finalized and incorporated blocks.
// "Incorporated" means a block connects to a given block through a certain number of intermediate blocks
// Note: the descendants doesn't include the blockID itself.
func FindDescendants(height uint64, blockID flow.Identifier, descendants *[]flow.Identifier) func(*badger.Txn) error {
	// height + 1 to exclude the blockID, and all blocks at the same height
	start, end := payloadIterRange(codeIndexGuarantee, height+1, math.MaxUint64)
	return iterate(start, end, keyonly(finddescendant(blockID, descendants)))
}
