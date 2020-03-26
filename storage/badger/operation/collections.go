// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// NOTE: These insert light collections, which only contain references
// to the constituent transactions. They do not modify transactions contained
// by the collections.

func InsertCollection(collection *flow.LightCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeCollection, collection.ID()), collection)
}

func CheckCollection(collID flow.Identifier, exists *bool) func(*badger.Txn) error {
	return check(makePrefix(codeCollection, collID), exists)
}

func RetrieveCollection(collID flow.Identifier, collection *flow.LightCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeCollection, collID), collection)
}

func RemoveCollection(collID flow.Identifier) func(*badger.Txn) error {
	return remove(makePrefix(codeCollection, collID))
}

// IndexCollection indexes the transactions within the collection payload of a block.
func IndexCollectionPayload(height uint64, blockID, parentID flow.Identifier, collection *flow.LightCollection) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollection, height, blockID, parentID), collection.Transactions)
}

// LookupCollection looks up the collection for a given payload.
func LookupCollectionPayload(height uint64, blockID, parentID flow.Identifier, collection *flow.LightCollection) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCollection, height, blockID, parentID), &collection.Transactions)
}

// VerifyCollectionPayload verifies that the candidate transaction IDs
//// don't exist in any ancestor block.
func VerifyCollectionPayload(height uint64, blockID flow.Identifier, txIDs []flow.Identifier) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexCollection, height), makePrefix(codeIndexCollection, uint64(0)), validatepayload(blockID, txIDs))
}

// CheckCollectionPayload populates `invalidIDs` with any IDs in the candidate
// set that already exist in an ancestor block.
func CheckCollectionPayload(height uint64, blockID flow.Identifier, candidateIDs []flow.Identifier, invalidIDs *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return iterate(makePrefix(codeIndexCollection, height), makePrefix(codeIndexCollection, uint64(0)), searchduplicates(blockID, candidateIDs, invalidIDs))
}

// IndexCollectionByTransaction inserts a collection id keyed by a transaction id
func IndexCollectionByTransaction(txID, collectionID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionID retrieves a collection id by transaction id
func RetrieveCollectionID(txID flow.Identifier, collectionID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}
