package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ProtocolKVStore persists different snapshots of key-value stores [KV-stores]. At this level, the API
// deals with versioned data blobs, each representing a Snapshot of the Protocol State. The *current*
// implementation allows to retrieve snapshots from the database (e.g. to answer external API calls) even
// for legacy protocol states whose versions are not support anymore. However, this _may_ change in the
// future, where only versioned snapshots can be retrieved that are also supported by the current software.
// TODO maybe rename to `ProtocolStateSnapshots` (?) because at this low level, we are not exposing the
// KV-store, it is just an encoded data blob
type ProtocolKVStore interface {
	// BatchStore persists the KV-store snapshot in the database using the given ID as key.
	// BatchStore is idempotent, i.e. it accepts repeated calls with the same pairs of (stateID, kvStore).
	// Here, the ID is expected to be a collision-resistant hash of the snapshot (including the
	// ProtocolStateVersion).
	//
	// No error is expected during normal operations.
	BatchStore(rw ReaderBatchWriter, stateID flow.Identifier, data *flow.PSKeyValueStoreData) error

	// BatchIndex appends the following operation to the provided write batch:
	// we extend the map from `blockID` to `stateID`, where `blockID` references the
	// block that _proposes_ updated key-value store.
	// BatchIndex is idempotent, i.e. it accepts repeated calls with the same pairs of (blockID , stateID).
	// Per protocol convention, the block references the `stateID`. As the `blockID` is a collision-resistant hash,
	// for the same `blockID`, BatchIndex will reject changing the data.
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
	// Expected error returns during normal operations:
	// - [storage.ErrAlreadyExists] if a KV store for the given blockID has already been indexed
	BatchIndex(lctx lockctx.Proof, rw ReaderBatchWriter, blockID flow.Identifier, stateID flow.Identifier) error

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
