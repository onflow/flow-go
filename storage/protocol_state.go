package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState represents persistent storage for protocol state entries.
type ProtocolState interface {
	// StoreTx allows us to store protocol state as part of a DB tx, while still going through the caching layer.
	StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error
	// Index indexes the protocol state by block ID.
	Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error
	// ByID returns the protocol state by its ID.
	ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error)
	// ByBlockID returns the protocol state by block ID.
	ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error)
}
