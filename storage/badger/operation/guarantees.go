package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertGuarantee inserts a collection guarantee by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertGuarantee(guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return insert(makePrefix(codeGuarantee, guaranteeID), guarantee)
}

// RetrieveGuarantee retrieves a collection guarantee by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveGuarantee(guaranteeID flow.Identifier, guarantee *flow.CollectionGuarantee) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuarantee, guaranteeID), guarantee)
}

// IndexGuarantee indexes a collection guarantee by collection ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer
func IndexGuarantee(collectionID flow.Identifier, guaranteeID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
}

// LookupGuarantee finds collection guarantee ID by collection ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupGuarantee(collectionID flow.Identifier, guaranteeID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeGuaranteeByCollectionID, collectionID), guaranteeID)
}

// IndexPayloadGuarantees indexes a collection guarantees by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer
func IndexPayloadGuarantees(blockID flow.Identifier, guarIDs []flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}

// LookupPayloadGuarantees finds collection guarantees by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupPayloadGuarantees(blockID flow.Identifier, guarIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codePayloadGuarantees, blockID), guarIDs)
}
