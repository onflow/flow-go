package store

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// DefaultProtocolKVStoreCacheSize is the default size for primary protocol KV store cache.
// KV store is rarely updated, so we will have a limited number of unique snapshots.
// Let's be generous and assume we have 10 different KV stores used at the same time.
var DefaultProtocolKVStoreCacheSize uint = 10

// DefaultProtocolKVStoreByBlockIDCacheSize is the default value for secondary index `byBlockIdCache`.
// We want to be able to cover a broad interval of views without cache misses, so we use a bigger value.
// Generally, many blocks will reference the same KV store snapshot.
var DefaultProtocolKVStoreByBlockIDCacheSize uint = 1000

// ProtocolKVStore implements persistent storage for storing KV store snapshots.
type ProtocolKVStore struct {
	db storage.DB

	// cache holds versioned binary blobs representing snapshots of key-value stores. We use the kv-store's
	// ID as key for retrieving the versioned binary snapshot of the kv-store. Consumers must know how to
	// deal with the binary representation. `cache` only holds the distinct snapshots. On the happy path,
	// we expect single-digit number of unique snapshots within an epoch.
	cache *Cache[flow.Identifier, *flow.PSKeyValueStoreData]

	// byBlockIdCache is essentially an in-memory map from `Block.ID()` -> `KeyValueStore.ID()`. The full
	// kv-store snapshot can be retrieved from the `cache` above.
	// `byBlockIdCache` will contain an entry for every block. We want to be able to cover a broad interval of views
	// without cache misses, so a cache size of roughly 1000 entries is reasonable.
	byBlockIdCache *Cache[flow.Identifier, flow.Identifier]
}

var _ storage.ProtocolKVStore = (*ProtocolKVStore)(nil)

// NewProtocolKVStore creates a ProtocolKVStore instance, which is a database holding KV store snapshots.
// It supports storing, caching and retrieving by ID or the additionally indexed block ID.
func NewProtocolKVStore(collector module.CacheMetrics,
	db storage.DB,
	kvStoreCacheSize uint,
	kvStoreByBlockIDCacheSize uint,
) *ProtocolKVStore {
	retrieveByStateID := func(r storage.Reader, stateID flow.Identifier) (*flow.PSKeyValueStoreData, error) {
		var kvStore flow.PSKeyValueStoreData
		err := operation.RetrieveProtocolKVStore(r, stateID, &kvStore)
		if err != nil {
			return nil, fmt.Errorf("could not get kv snapshot by id (%x): %w", stateID, err)
		}
		return &kvStore, nil
	}
	storeByStateID := func(rw storage.ReaderBatchWriter, stateID flow.Identifier, data *flow.PSKeyValueStoreData) error {
		return operation.InsertProtocolKVStore(rw.Writer(), stateID, data)
	}

	storeByBlockID := func(rw storage.ReaderBatchWriter, blockID flow.Identifier, stateID flow.Identifier) error {
		err := operation.IndexProtocolKVStore(rw.Writer(), blockID, stateID)
		if err != nil {
			return fmt.Errorf("could not index protocol state for block (%x): %w", blockID[:], err)
		}
		return nil
	}

	retrieveByBlockID := func(r storage.Reader, blockID flow.Identifier) (flow.Identifier, error) {
		var stateID flow.Identifier
		err := operation.LookupProtocolKVStore(r, blockID, &stateID)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
		}
		return stateID, nil
	}

	return &ProtocolKVStore{
		db: db,
		cache: newCache(collector, metrics.ResourceProtocolKVStore,
			withLimit[flow.Identifier, *flow.PSKeyValueStoreData](kvStoreCacheSize),
			withStore(storeByStateID),
			withRetrieve(retrieveByStateID)),
		byBlockIdCache: newCache(collector, metrics.ResourceProtocolKVStoreByBlockID,
			withLimit[flow.Identifier, flow.Identifier](kvStoreByBlockIDCacheSize),
			withStore(storeByBlockID),
			withRetrieve(retrieveByBlockID)),
	}
}

// BatchStore persists the KV-store snapshot in the database using the given ID as key.
// BatchStore is idempotent, i.e. it accepts repeated calls with the same pairs of (stateID, kvStore).
// Here, the ID is expected to be a collision-resistant hash of the snapshot (including the
// ProtocolStateVersion). Hence, for the same ID, BatchStore will reject changing the data.
// Expected errors during normal operations:
// - [storage.ErrDataMismatch] if a _different_ KV store for the given stateID has already been persisted
func (s *ProtocolKVStore) BatchStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, stateID flow.Identifier, data *flow.PSKeyValueStoreData) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	existingData, err := s.ByID(stateID)
	if err == nil {
		if existingData.Equal(data) {
			return nil
		}

		return fmt.Errorf("kv-store snapshot with id (%x) already exists but different, ([%v,%x] != [%v,%x]): %w", stateID[:],
			data.Version, data.Data,
			existingData.Version, existingData.Data,
			storage.ErrDataMismatch)
	}
	if !errors.Is(err, storage.ErrNotFound) { // `storage.ErrNotFound` is expected, as this indicates that no receipt is indexed yet; anything else is an exception
		return fmt.Errorf("unexpected error checking if kv-store snapshot %x exists: %w", stateID[:], irrecoverable.NewException(err))
	}

	return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return s.cache.PutTx(rw, stateID, data)
	})
}

// BatchIndex persists the specific map entry in the node's database.
// In a nutshell, we want to maintain a map from `blockID` to `epochStateEntry`, where `blockID` references the
// block that _proposes_ the referenced epoch protocol state entry.
// Protocol convention:
//   - Consider block B, whose ingestion might potentially lead to an updated protocol state. For example,
//     the protocol state changes if we seal some execution results emitting service events.
//   - For the key `blockID`, we use the identity of block B which _proposes_ this Protocol State. As value,
//     the hash of the resulting protocol state at the end of processing B is to be used.
//   - IMPORTANT: The protocol state requires confirmation by a QC and will only become active at the child block,
//     _after_ validating the QC.
//
// CAUTION:
//   - The caller must acquire the lock [storage.LockInsertBlock] and hold it until the database write has been committed.
//   - OVERWRITES existing data (potential for data corruption):
//     This method silently overrides existing data without any sanity checks whether data for the same key already exits.
//     Note that the Flow protocol mandates that for a previously persisted key, the data is never changed to a different
//     value. Changing data could cause the node to publish inconsistent data and to be slashed, or the protocol to be
//     compromised as a whole. This method does not contain any safeguards to prevent such data corruption. The lock proof
//     serves as a reminder that the CALLER is responsible to ensure that the DEDUPLICATION CHECK is done elsewhere
//     ATOMICALLY with this write operation.
//
// Expected errors during normal operations:
// - [storage.ErrDataMismatch] if a _different_ KV store for the given stateID has already been persisted
func (s *ProtocolKVStore) BatchIndex(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockID flow.Identifier, stateID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockInsertBlock) {
		return fmt.Errorf("missing required lock: %s", storage.LockInsertBlock)
	}

	existingStateID, err := s.byBlockIdCache.Get(s.db.Reader(), blockID)
	if err == nil {
		// no-op if the *same* stateID is already indexed for the blockID
		if existingStateID == stateID {
			return nil
		}

		return fmt.Errorf("kv-store snapshot with block id (%x) already exists and different (%v != %v): %w",
			blockID[:],
			stateID, existingStateID,
			storage.ErrDataMismatch)
	}
	if !errors.Is(err, storage.ErrNotFound) { // `storage.ErrNotFound` is expected, as this indicates that no receipt is indexed yet; anything else is an exception
		return fmt.Errorf("could not check if kv-store snapshot with block id (%x) exists: %w", blockID[:], irrecoverable.NewException(err))
	}

	return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return s.byBlockIdCache.PutTx(rw, blockID, stateID)
	})
}

// ByID retrieves the KV store snapshot with the given state ID.
// Expected errors during normal operations:
//   - storage.ErrNotFound if no snapshot with the given Identifier is known.
func (s *ProtocolKVStore) ByID(stateID flow.Identifier) (*flow.PSKeyValueStoreData, error) {
	return s.cache.Get(s.db.Reader(), stateID)
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
func (s *ProtocolKVStore) ByBlockID(blockID flow.Identifier) (*flow.PSKeyValueStoreData, error) {
	stateID, err := s.byBlockIdCache.Get(s.db.Reader(), blockID)
	if err != nil {
		return nil, fmt.Errorf("could not lookup protocol state ID for block (%x): %w", blockID[:], err)
	}
	return s.cache.Get(s.db.Reader(), stateID)
}
