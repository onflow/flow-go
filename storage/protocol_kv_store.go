package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolKVStore persists different snapshots of key-value stores [KV-stores]. At this level, the API
// deals with versioned data blobs, each representing a Snapshot of the Protocol State. The *current*
// implementation allows to retrieve snapshots from the database (e.g. to answer external API calls) even
// for legacy protocol states whose versions are not support anymore. However, this _may_ change in the
// future, where only versioned snapshots can be retrieved that are also supported by the current software.
// TODO maybe rename to `ProtocolStateSnapshots` (?) because at this low level, we are not exposing the
// KV-store, it is just an encoded data blob
type ProtocolKVStore interface {
	// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
	// which persists the given KV-store snapshot as part of a DB tx.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a KV-store snapshot with the given id is already stored.
	StoreTx(stateID flow.Identifier, data *flow.PSKeyValueStoreData) func(*transaction.Tx) error

	// IndexTx returns an anonymous function intended to be executed as part of a database transaction.
	// In a nutshell, we want to maintain a map from `blockID` to `stateID`, where `blockID` references the
	// block that _proposes_ the updated key-value store.
	// Upon call, the anonymous function persists the specific map entry in the node's database.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated KV store. For example,
	//     the KV store changes if we seal some execution results emitting specific service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this updated KV store.
	//   - CAUTION: The updated state requires confirmation by a QC and will only become active at the
	//     child block, _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrAlreadyExists if a KV store for the given blockID has already been indexed.
	IndexTx(blockID flow.Identifier, stateID flow.Identifier) func(*transaction.Tx) error

	// ByID retrieves the KV store snapshot with the given ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no snapshot with the given Identifier is known.
	ByID(id flow.Identifier) (*flow.PSKeyValueStoreData, error)

	// ByBlockID retrieves the kv-store snapshot that the block with the given ID proposes.
	// CAUTION: this store snapshot requires confirmation by a QC and will only become active at the child block,
	// _after_ validating the QC. Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated KV store state.
	//     For example, the state changes if we seal some execution results emitting specific service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this updated KV store. As value,
	//     the hash of the resulting state at the end of processing B is to be used.
	//   - CAUTION: The updated state requires confirmation by a QC and will only become active at the child block,
	//     _after_ validating the QC.
	//
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no snapshot has been indexed for the given block.
	ByBlockID(blockID flow.Identifier) (*flow.PSKeyValueStoreData, error)
}
