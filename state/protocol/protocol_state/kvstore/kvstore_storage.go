package kvstore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
)

// ProtocolKVStore persists different snapshots of key-value stores [KV-stores]. Here, we augment
// the low-level primitives provided by `storage.ProtocolKVStore` with logic for encoding and
// decoding the state snapshots into abstract representation `protocol_state.KVStoreAPI`.
//
// TODO (optional): include a cache of the _decoded_ Protocol States, so we don't decode+encode on each consensus view (hot-path)
type ProtocolKVStore struct {
	storage.ProtocolKVStore
}

var _ protocol_state.ProtocolKVStore = (*ProtocolKVStore)(nil)

// NewProtocolKVStore instantiates a ProtocolKVStore for querying & storing deserialized `protocol_state.KVStoreAPIs`.
// At this abstraction level, we can only handle protocol state snapshots, whose data models are supported by the current
// software version. There might be serialized snapshots with legacy versions in the database, that are not supported
// anymore by this software version and can only be retrieved as versioned binary blobs via `storage.ProtocolKVStore`.
func NewProtocolKVStore(protocolStateSnapshots storage.ProtocolKVStore) *ProtocolKVStore {
	return &ProtocolKVStore{
		ProtocolKVStore: protocolStateSnapshots,
	}
}

// BatchStore adds the KV-store snapshot in the database using the given ID as key. Per convention, all
// implementations of [protocol.KVStoreReader] should be able to successfully encode their state into a
// data blob. If the encoding fails, an error is returned.
// BatchStore is idempotent, i.e. it accepts repeated calls with the same pairs of (stateID, kvStore).
// Here, the ID is expected to be a collision-resistant hash of the snapshot (including the
// ProtocolStateVersion).
//
// No error is exepcted during normal operations
func (p *ProtocolKVStore) BatchStore(rw storage.ReaderBatchWriter, stateID flow.Identifier, kvStore protocol.KVStoreReader) error {
	version, data, err := kvStore.VersionedEncode()
	if err != nil {
		return fmt.Errorf("failed to VersionedEncode protocol state: %w", err)
	}
	return p.ProtocolKVStore.BatchStore(rw, stateID, &flow.PSKeyValueStoreData{
		Version: version,
		Data:    data,
	})
}

// ByID retrieves the KV store snapshot with the given ID.
// Expected errors during normal operations:
//   - storage.ErrNotFound if no snapshot with the given Identifier is known.
//   - ErrUnsupportedVersion if input version is not supported
func (p *ProtocolKVStore) ByID(protocolStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	versionedData, err := p.ProtocolKVStore.ByID(protocolStateID)
	if err != nil {
		return nil, fmt.Errorf("could not query KV store with ID %x: %w", protocolStateID, err)
	}
	kvStore, err := VersionedDecode(versionedData.Version, versionedData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode protocol state (version=%d) with ID %x: %w", versionedData.Version, protocolStateID, err)
	}
	return kvStore, err
}

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
func (p *ProtocolKVStore) ByBlockID(blockID flow.Identifier) (protocol_state.KVStoreAPI, error) {
	versionedData, err := p.ProtocolKVStore.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not query KV store at block (%x): %w", blockID, err)
	}
	kvStore, err := VersionedDecode(versionedData.Version, versionedData.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decode protocol state (version=%d) at block (%x): %w", versionedData.Version, blockID, err)
	}
	return kvStore, err
}
