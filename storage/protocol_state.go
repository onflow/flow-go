package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState represents persistent storage for protocol state entries.
// TODO: update naming to IdentityTable
type ProtocolState interface {
	// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
	// which persists the given protocol state as part of a DB tx.
	// Per convention, the given Identity Table must be in canonical order, otherwise an exception is returned.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a protocol state snapshot has already been stored under the given id.
	StoreTx(id flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error

	// Index returns an anonymous function that is intended to be executed as part of a database transaction.
	// In a nutshell, we want to maintain a map from `blockID` to `protocolStateID`.
	// Upon call, the anonymous function persists the specific map entry in the node's database.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B. As value, the hash of the resulting protocol
	//     state at the end of processing B is to be used.
	// Expected errors during normal operations:
	//   - storage.ErrAlreadyExists if the key already exists in the database.
	Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error

	// ByID returns the protocol state by its ID.
	// Expected errors during normal operations:
	//  * `storage.ErrNotFound` if no protocol state snapshot with the given Identifier is known.
	ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error)

	// ByBlockID retrieves the identity table by the respective block ID.
	// TODO: clarify whether the blockID is the block that defines this identity table or the _child_ block where the identity table is applied. CAUTION: surface for bugs!
	// Expected errors during normal operations:
	//  * `storage.ErrNotFound` if no snapshot of the protocol state has been indexed for the given block.
	ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error)
}
