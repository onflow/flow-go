package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// KeyValueStoreData is a binary blob with a version attached, specifying the format
// of the marshaled data. In a nutshell, it serves as a binary snapshot of a ProtocolKVStore.
// This structure is useful for version-agnostic storage, where snapshots with different versions
// can co-exist. The KeyValueStoreData is a generic format that can be later decoded to
// potentially different strongly typed structures based on version. When reading from the store,
// we rely on the fact that caller knows how to deal with the binary representation.
type KeyValueStoreData struct {
	Version uint64
	Data    []byte
}

// ProtocolKVStore persists different snapshots of key-value stores.
type ProtocolKVStore interface {
	// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
	// which persists the given model as part of a DB tx.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a model with the given id is already stored
	StoreTx(stateID flow.Identifier, data *KeyValueStoreData) func(*transaction.Tx) error

	// IndexTx returns an anonymous function intended to be executed as part of a database transaction.
	// In a nutshell, we want to maintain a map from `blockID` to `stateID`, where `blockID` references the
	// block that _proposes_ updated key-value store.
	// Upon call, the anonymous function persists the specific map entry in the node's database.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
	//     the protocol state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
	//     the hash of the resulting key-value store state at the end of processing B is to be used.
	//   - CAUTION: The updated state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrAlreadyExists if a Protocol State for the given blockID has already been indexed
	IndexTx(blockID flow.Identifier, stateID flow.Identifier) func(*transaction.Tx) error

	// ByID returns the key-value store model by its ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no model with the given Identifier is known.
	ByID(id flow.Identifier) (*KeyValueStoreData, error)

	// ByBlockID retrieves the key-value store model that the block with the given ID proposes.
	// CAUTION: this store state requires confirmation by a QC and will only become active at the child block,
	// _after_ validating the QC. Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated store state.
	// For example, the state changes if we seal some execution results emitting service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this store. As value,
	//     the hash of the resulting state at the end of processing B is to be used.
	//   - CAUTION: The updated state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no model has been indexed for the given block.
	ByBlockID(blockID flow.Identifier) (*ProtocolKVStore, error)
}
