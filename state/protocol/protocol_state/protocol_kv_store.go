package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// ProtocolKVStore persists different snapshots of key-value stores [KV-stores]. Here, we augment
// the low-level primitives provided by `storage.ProtocolKVStore` with logic for encoding and
// decoding the state snapshots into abstract representation `protocol_state.KVStoreAPI`.
type ProtocolKVStore interface {
	// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
	// which persists the given KV-store snapshot as part of a DB tx. Per convention, all implementations
	// of `protocol.KVStoreReader` should be able to successfully  encode their state into a data blob.
	// Expected errors of the returned anonymous function:
	//   - storage.ErrAlreadyExists if a KV-store snapshot with the given id is already stored.
	StoreTx(stateID flow.Identifier, kvStore protocol.KVStoreReader) func(*transaction.Tx) error

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
	//   - ErrUnsupportedVersion if input version is not supported
	ByID(id flow.Identifier) (KVStoreAPI, error)

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
	//   - ErrUnsupportedVersion if input version is not supported
	ByBlockID(blockID flow.Identifier) (KVStoreAPI, error)
}
