package store

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
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
	// storing        *sync.Mutex
	// indexing       *sync.Mutex
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
		// storing: new(sync.Mutex),
		// indexing: new(sync.Mutex),
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

// StoreTx returns an anonymous function (intended to be executed as part of a badger transaction),
// which persists the given KV-store snapshot as part of a DB tx.
// Expected errors of the returned anonymous function:
//   - storage.ErrAlreadyExists if a KV-store snapshot with the given id is already stored.
func (s *ProtocolKVStore) StoreTx(stateID flow.Identifier, data *flow.PSKeyValueStoreData) func(*transaction.Tx) error {
	panic("not implemented")
}

func (s *ProtocolKVStore) BatchStore(rw storage.ReaderBatchWriter, stateID flow.Identifier, data *flow.PSKeyValueStoreData) error {
	// TOOD: add synchronization, adding lock would cause deadlock during bootstrap
	// s.storing.Lock()
	// rw.AddCallback(func(error) {
	// 	s.storing.Unlock()
	// })

	_, err := s.ByID(stateID)
	if err == nil {
		return fmt.Errorf("kv-store snapshot with id (%x) already exists: %w", stateID[:], storage.ErrAlreadyExists)
	}

	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check if kv-store snapshot with id (%x) exists: %w", stateID[:], err)
	}

	return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return s.cache.PutTx(rw, stateID, data)
	})
}

// IndexTx returns an anonymous function intended to be executed as part of a database transaction.
// In a nutshell, we want to maintain a map from `blockID` to `stateID`, where `blockID` references the
// block that _proposes_ updated key-value store.
// Upon call, the anonymous function persists the specific map entry in the node's database.
// Protocol convention:
//   - Consider block B, whose ingestion might potentially lead to an updated KV store. For example,
//     the KV store changes if we seal some execution results emitting specific service events.
//   - For the key `blockID`, we use the identity of block B which _proposes_ this updated KV store.
//   - CAUTION: The updated state requires confirmation by a QC and will only become active at the child block,
//     _after_ validating the QC.
//
// Expected errors during normal operations:
//   - storage.ErrAlreadyExists if a KV store for the given blockID has already been indexed.
func (s *ProtocolKVStore) IndexTx(blockID flow.Identifier, stateID flow.Identifier) func(*transaction.Tx) error {
	panic("not implemented")
}

func (s *ProtocolKVStore) BatchIndex(rw storage.ReaderBatchWriter, blockID flow.Identifier, stateID flow.Identifier) error {
	// TODO: add synchronization, adding lock would cause deadlock during bootstrap
	// s.indexing.Lock()
	// rw.AddCallback(func(error) {
	// 	s.indexing.Unlock()
	// })

	_, err := s.byBlockIdCache.Get(s.db.Reader(), blockID)
	if err == nil {
		return fmt.Errorf("kv-store snapshot with block id (%x) already exists: %w", blockID[:], storage.ErrAlreadyExists)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check if kv-store snapshot with block id (%x) exists: %w", blockID[:], err)
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
