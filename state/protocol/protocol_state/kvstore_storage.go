package protocol_state

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ProtocolKVStore persists different snapshots of the Protocol State's Key-Calue stores [KV-stores].
// Here, we augment the low-level primitives provided by `storage.ProtocolKVStore` with logic for
// encoding and decoding the state snapshots into abstract representation `protocol_state.KVStoreAPI`.
//
// At the abstraction level here, we can only handle protocol state snapshots, whose data models are
// supported by the current software version. There might be serialized snapshots with legacy versions
// in the database that are not supported anymore by this software version.
type ProtocolKVStore interface {
	// BatchStore adds the KV-store snapshot in the database using the given ID as key. Per convention, all
	// implementations of [protocol.KVStoreReader] should be able to successfully encode their state into a
	// data blob. If the encoding fails, an error is returned.
	//
	// No error is expected during normal operations
	BatchStore(rw storage.ReaderBatchWriter, stateID flow.Identifier, kvStore protocol.KVStoreReader) error

	// BatchIndex writes the blockID->stateID index to the input write batch.
	// In a nutshell, we want to maintain a map from `blockID` to `stateID`, where `blockID` references the
	// block that _proposes_ the updated key-value store.
	// Protocol convention:
	//   - Consider block B, whose ingestion might potentially lead to an updated KV store. For example,
	//     the KV store changes if we seal some execution results emitting specific service events.
	//   - For the key `blockID`, we use the identity of block B which _proposes_ this updated KV store.
	//   - IMPORTANT: The updated state requires confirmation by a QC and will only become active at the
	//     child block, _after_ validating the QC.
	//
	// CAUTION: To prevent data corruption, we need to guarantee atomicity of existence-check and the subsequent
	// database write. Hence, we require the caller to acquire [storage.LockInsertBlock] and hold it until the
	// database write has been committed.
	//
	// Expected errors of the returned anonymous function:
	//   - [storage.ErrAlreadyExists] if a KV store for the given blockID has already been indexed.
	BatchIndex(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, stateID flow.Identifier) error

	// ByID retrieves the KV store snapshot with the given ID.
	// Expected errors during normal operations:
	//   - storage.ErrNotFound if no snapshot with the given Identifier is known.
	//   - kvstore.ErrUnsupportedVersion if the version of the stored snapshot not supported by this implementation
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
	//   - kvstore.ErrUnsupportedVersion if the version of the stored snapshot not supported by this implementation
	ByBlockID(blockID flow.Identifier) (KVStoreAPI, error)
}
