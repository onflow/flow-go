package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolState represents persistent storage for protocol state entries.
type ProtocolState interface {
	// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
	// which persists the given protocol state as part of a DB tx. Per convention, the identities in
	// the Protocol State must be in canonical order for the current and next epoch (if present),
	// otherwise an exception is returned.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a Protocol State with the given id is already stored
	StoreTx(protocolStateID flow.Identifier, protocolState *flow.ProtocolStateEntry) func(*transaction.Tx) error

	// Index returns an anonymous function that is intended to be executed as part of a database transaction.
	// In a nutshell, we want to maintain a map from `blockID` to `protocolStateID`, where `blockID` references the
	// block that _proposes_ the Protocol State.
	// Upon call, the anonymous function persists the specific map entry in the node's database.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
	//     the hash of the resulting protocol state at the end of processing B is to be used.
	//   - CAUTION: The protocol state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrAlreadyExists if a Protocol State for the given blockID has already been indexed
	Index(blockID flow.Identifier, protocolStateID flow.Identifier) func(*transaction.Tx) error

	// ByID returns the protocol state by its ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no protocol state with the given Identifier is known.
	ByID(id flow.Identifier) (*flow.RichProtocolStateEntry, error)

	// ByBlockID retrieves the Protocol State that the block with the given ID proposes.
	// CAUTION: this protocol state requires confirmation by a QC and will only become active at the child block,
	// _after_ validating the QC. Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
	//     the hash of the resulting protocol state at the end of processing B is to be used.
	//   - CAUTION: The protocol state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no protocol state has been indexed for the given block.
	ByBlockID(blockID flow.Identifier) (*flow.RichProtocolStateEntry, error)
}
