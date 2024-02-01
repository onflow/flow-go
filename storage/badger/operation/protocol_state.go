package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertProtocolState inserts a protocol state by ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func InsertProtocolState(protocolStateID flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolState, protocolStateID), protocolState)
}

// RetrieveProtocolState retrieves a protocol state by ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func RetrieveProtocolState(protocolStateID flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolState, protocolStateID), protocolState)
}

// IndexProtocolState indexes a protocol state by block ID.
// Error returns:
//   - storage.ErrAlreadyExists if the key already exists in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure.
func IndexProtocolState(blockID flow.Identifier, protocolStateID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolStateByBlockID, blockID), protocolStateID)
}

// LookupProtocolState finds protocol state ID by block ID.
// Error returns:
//   - storage.ErrNotFound if the key does not exist in the database
//   - generic error in case of unexpected failure from the database layer
func LookupProtocolState(blockID flow.Identifier, protocolStateID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolStateByBlockID, blockID), protocolStateID)
}
