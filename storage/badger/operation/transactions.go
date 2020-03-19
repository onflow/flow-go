package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

// InsertTransaction inserts a transaction keyed by transaction fingerprint.
func InsertTransaction(tx *flow.TransactionBody) func(*badger.Txn) error {
	return insert(makePrefix(codeTransaction, tx.ID()), tx)
}

// RetrieveTransaction retrieves a transaction by fingerprint.
func RetrieveTransaction(txID flow.Identifier, tx *flow.TransactionBody) func(*badger.Txn) error {
	return retrieve(makePrefix(codeTransaction, txID), tx)
}

// IndexCollectionByTransaction inserts a collection id keyed by a transaction id
func IndexCollectionByTransaction(txID, collectionID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}

// LookupCollectionID retrieves a collection id by transaction id
func LookupCollectionID(txID flow.Identifier, collectionID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexCollectionByTransaction, txID), collectionID)
}
