package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagemodel "github.com/onflow/flow-go/storage/badger/model"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type ChunkDataPacks struct {
	db             *badger.DB
	byChunkIDCache *Cache
}

func NewChunkDataPacks(collector module.CacheMetrics, db *badger.DB, byChunkIDCacheSize uint) *ChunkDataPacks {

	store := func(key interface{}, val interface{}) func(*transaction.Tx) error {
		chdp := val.(*storagemodel.StoredChunkDataPack)
		return transaction.WithTx(operation.SkipDuplicates(operation.InsertChunkDataPack(chdp)))
	}

	retrieve := func(key interface{}) func(tx *badger.Txn) (interface{}, error) {
		chunkID := key.(flow.Identifier)

		var c storagemodel.StoredChunkDataPack
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

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*storagemodel.StoredChunkDataPack, error) {
	tx := ch.db.NewTransaction(false)
	defer tx.Discard()
	return ch.retrieveCHDP(chunkID)(tx)
}

func (ch *ChunkDataPacks) retrieveCHDP(chunkID flow.Identifier) func(*badger.Txn) (*storagemodel.StoredChunkDataPack, error) {
	return func(tx *badger.Txn) (*storagemodel.StoredChunkDataPack, error) {
		val, err := ch.byChunkIDCache.Get(chunkID)(tx)
		if err != nil {
			return nil, err
		}
		return val.(*storagemodel.StoredChunkDataPack), nil
	}
}

func toStoredChunkDataPack(c *flow.ChunkDataPack) *storagemodel.StoredChunkDataPack {
	sc := &storagemodel.StoredChunkDataPack{
		ChunkID:    c.ChunkID,
		StartState: c.StartState,
		Proof:      c.Proof,
	}

	if c.Collection != nil {
		// non-system chunks have a non-nil collection
		collectionID := c.Collection.ID()
		sc.CollectionID = &collectionID
	}

	return sc
}
