package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState represents persistent storage for protocol state entries.
// TODO: update naming to IdentityTable
type ProtocolState interface {
	// StoreTx allows us to store an identity table as part of a DB tx, while still going through the caching layer.
	// Per convention, the given Identity Table must be in canonical order, otherwise an exception is returned.
	// Expected error returns during normal operations:
	//   - storage.ErrAlreadyExists if an Identity Table with the given id is already stored
	StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error

	// Index indexes the identity table by block ID.
	// Error returns:
	//   - storage.ErrAlreadyExists if an identity table for the given blockID has already been indexed
	Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error

	// ByID retrieves the identity table by its ID.
	// Error returns:
	//   - storage.ErrNotFound if no identity table with the given ID exists
	ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error)

	// ByBlockID retrieves the identity table by the respective block ID.
	// TODO: clarify whether the blockID is the block that defines this identity table or the _child_ block where the identity table is applied. CAUTION: surface for bugs!
	// Error returns:
	//   - storage.ErrNotFound if no identity table for the given blockID exists
	ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error)
}
