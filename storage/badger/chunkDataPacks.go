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
	byChunkIDCache *Cache
}

func NewChunkDataPacks(collector module.CacheMetrics, db *badger.DB, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		chdp := val.(*badgermodel.StoredChunkDataPack)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertChunkDataPack(chdp)))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		chunkID := key.(flow.Identifier)

		var c badgermodel.StoredChunkDataPack
		return func(tx *badger.Txn) (interface{}, error) {
			err := operation.RetrieveChunkDataPack(chunkID, &c)(tx)
			return &c, err
		}
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit(byChunkIDCacheSize),
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

func (ch *ChunkDataPacks) Store(c *flow.ChunkDataPack) error {
	sc := toStoredChunkDataPack(c)
	err := operation.RetryOnConflictTx(ch.db, transaction.Update, ch.byChunkIDCache.PutTx(sc.ChunkID, sc))
	if err != nil {
		return fmt.Errorf("could not store chunk datapack: %w", err)
	}
	return nil
}

func (ch *ChunkDataPacks) Remove(chunkID flow.Identifier) error {
	err := operation.RetryOnConflict(ch.db.Update, operation.RemoveChunkDataPack(chunkID))
	if err != nil {
		return fmt.Errorf("could not remove chunk datapack: %w", err)
	}
	// TODO Integrate cache removal in a similar way as storage/retrieval is
	ch.byChunkIDCache.Remove(chunkID)
	return nil
}

func (ch *ChunkDataPacks) BatchStore(c *flow.ChunkDataPack, batch storage.BatchStorage) error {
	sc := toStoredChunkDataPack(c)
	writeBatch := batch.GetWriter()
	batch.OnSucceed(func() {
		ch.byChunkIDCache.Insert(sc.ChunkID, sc)
	})
	return operation.BatchInsertChunkDataPack(sc)(writeBatch)
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	schdp, err := ch.byChunkID(chunkID)
	if err != nil {
		return nil, err
	}

	chdp := &flow.ChunkDataPack{
		ChunkID:    schdp.ChunkID,
		StartState: schdp.StartState,
		Proof:      schdp.Proof,
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
		return val.(*badgermodel.StoredChunkDataPack), nil
	}
}

func toStoredChunkDataPack(c *flow.ChunkDataPack) *badgermodel.StoredChunkDataPack {
	sc := &badgermodel.StoredChunkDataPack{
		ChunkID:     c.ChunkID,
		StartState:  c.StartState,
		Proof:       c.Proof,
		SystemChunk: false,
	}

	if c.Collection != nil {
		// non system chunk
		sc.CollectionID = c.Collection.ID()
	} else {
		sc.SystemChunk = true
	}

	return sc
}
