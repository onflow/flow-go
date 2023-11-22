package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	badgermodel "github.com/onflow/flow-go/storage/badger/model"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ChunkDataPacks struct {
	db             *badger.DB
	collections    storage.Collections
	byChunkIDCache *Cache[flow.Identifier, *badgermodel.StoredChunkDataPack]
}

func NewChunkDataPacks(collector module.CacheMetrics, db *badger.DB, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

	store := func(key flow.Identifier, val *badgermodel.StoredChunkDataPack) func(*transaction.Tx) error {
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertChunkDataPack(val)))
	}

	retrieve := func(key flow.Identifier) func(tx *badger.Txn) (*badgermodel.StoredChunkDataPack, error) {
		return func(tx *badger.Txn) (*badgermodel.StoredChunkDataPack, error) {
			var c badgermodel.StoredChunkDataPack
			err := operation.RetrieveChunkDataPack(key, &c)(tx)
			return &c, err
		}
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, *badgermodel.StoredChunkDataPack](byChunkIDCacheSize),
		withStore(store),
		withRetrieve(retrieve),
	)

	ch := ChunkDataPacks{
		db:             db,
		byChunkIDCache: cache,
		collections:    collections,
	}
	return &ch
}

// Remove removes multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, even if no entries are matched.
func (ch *ChunkDataPacks) Remove(chunkIDs []flow.Identifier) error {
	batch := NewBatch(ch.db)

	for _, c := range chunkIDs {
		err := ch.BatchRemove(c, batch)
		if err != nil {
			return fmt.Errorf("cannot remove chunk data pack: %w", err)
		}
	}

	err := batch.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush batch to remove chunk data pack: %w", err)
	}
	return nil
}

// BatchStore stores ChunkDataPack c keyed by its ChunkID in provided batch.
// No errors are expected during normal operation, but it may return generic error
// if entity is not serializable or Badger unexpectedly fails to process request
func (ch *ChunkDataPacks) BatchStore(c *flow.ChunkDataPack, batch storage.BatchStorage) error {
	sc := toStoredChunkDataPack(c)
	writeBatch := batch.GetWriter()
	batch.OnSucceed(func() {
		ch.byChunkIDCache.Insert(sc.ChunkID, sc)
	})
	return operation.BatchInsertChunkDataPack(sc)(writeBatch)
}

// Store stores multiple ChunkDataPacks cs keyed by their ChunkIDs in a batch.
// No errors are expected during normal operation, but it may return generic error
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) error {
	batch := NewBatch(ch.db)
	for _, c := range cs {
		err := ch.BatchStore(c, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}
	}

	err := batch.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush batch: %w", err)
	}
	return nil
}

// BatchRemove removes ChunkDataPack c keyed by its ChunkID in provided batch
// No errors are expected during normal operation, even if no entries are matched.
// If Badger unexpectedly fails to process the request, the error is wrapped in a generic error and returned.
func (ch *ChunkDataPacks) BatchRemove(chunkID flow.Identifier, batch storage.BatchStorage) error {
	writeBatch := batch.GetWriter()
	batch.OnSucceed(func() {
		ch.byChunkIDCache.Remove(chunkID)
	})
	return operation.BatchRemoveChunkDataPack(chunkID)(writeBatch)
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	schdp, err := ch.byChunkID(chunkID)
	if err != nil {
		return nil, err
	}

	chdp := &flow.ChunkDataPack{
		ChunkID:           schdp.ChunkID,
		StartState:        schdp.StartState,
		Proof:             schdp.Proof,
		ExecutionDataRoot: schdp.ExecutionDataRoot,
	}

	if !schdp.SystemChunk {
		collection, err := ch.collections.ByID(schdp.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not retrive collection (id: %x) for stored chunk data pack: %w", schdp.CollectionID, err)
		}

		chdp.Collection = collection
	}

	return chdp, nil
}

func (ch *ChunkDataPacks) byChunkID(chunkID flow.Identifier) (*badgermodel.StoredChunkDataPack, error) {
	tx := ch.db.NewTransaction(false)
	defer tx.Discard()

	schdp, err := ch.retrieveCHDP(chunkID)(tx)
	if err != nil {
		return nil, fmt.Errorf("could not retrive stored chunk data pack: %w", err)
	}

	return schdp, nil
}

func (ch *ChunkDataPacks) retrieveCHDP(chunkID flow.Identifier) func(*badger.Txn) (*badgermodel.StoredChunkDataPack, error) {
	return func(tx *badger.Txn) (*badgermodel.StoredChunkDataPack, error) {
		val, err := ch.byChunkIDCache.Get(chunkID)(tx)
		if err != nil {
			return nil, err
		}
		return val, nil
	}
}

func toStoredChunkDataPack(c *flow.ChunkDataPack) *badgermodel.StoredChunkDataPack {
	sc := &badgermodel.StoredChunkDataPack{
		ChunkID:           c.ChunkID,
		StartState:        c.StartState,
		Proof:             c.Proof,
		SystemChunk:       false,
		ExecutionDataRoot: c.ExecutionDataRoot,
	}

	if c.Collection != nil {
		// non system chunk
		sc.CollectionID = c.Collection.ID()
	} else {
		sc.SystemChunk = true
	}

	return sc
}
