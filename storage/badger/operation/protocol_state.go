package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
)

// InsertProtocolState inserts a protocol state by ID.
func InsertProtocolState(protocolStateID flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolState, protocolStateID), protocolState)
}

// RetrieveProtocolState retrieves a protocol state by ID.
func RetrieveProtocolState(protocolStateID flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolState, protocolStateID), protocolState)
}

// IndexProtocolState indexes a protocol state by block ID.
func IndexProtocolState(blockID flow.Identifier, protocolStateID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeProtocolStateByBlockID, blockID), protocolStateID)
}

// LookupProtocolState finds protocol state ID by block ID.
func LookupProtocolState(blockID flow.Identifier, protocolStateID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeProtocolStateByBlockID, blockID), protocolStateID)
}
