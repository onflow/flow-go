package kvstore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
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

// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction), which persists
// the given KV-store snapshot as part of a DB tx. Per convention, all implementations of `protocol.KVStoreReader`
// must support encoding their state into a version and data blob.
// Expected errors of the returned anonymous function:
//   - storage.ErrAlreadyExists if a KV-store snapshot with the given id is already stored.
func (p *ProtocolKVStore) StoreTx(stateID flow.Identifier, kvStore protocol.KVStoreReader) func(*transaction.Tx) error {
	version, data, err := kvStore.VersionedEncode()
	if err != nil {
		return func(*transaction.Tx) error {
			return fmt.Errorf("failed to VersionedEncode protocol state: %w", err)
		}
	}
	return p.ProtocolKVStore.StoreTx(stateID, &storage.KeyValueStoreData{
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
