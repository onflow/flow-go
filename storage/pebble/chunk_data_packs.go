package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type ChunkDataPacks struct {
	db             *pebble.DB
	collections    storage.Collections
	byChunkIDCache *Cache[flow.Identifier, *storage.StoredChunkDataPack]
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(collector module.CacheMetrics, db *pebble.DB, collections storage.Collections, byChunkIDCacheSize uint) *ChunkDataPacks {

	retrieve := func(key flow.Identifier) func(pebble.Reader) (*storage.StoredChunkDataPack, error) {
		return func(r pebble.Reader) (*storage.StoredChunkDataPack, error) {
			var c storage.StoredChunkDataPack
			err := operation.RetrieveChunkDataPack(key, &c)(r)
			return &c, err
		}
	}

	cache := newCache(collector, metrics.ResourceChunkDataPack,
		withLimit[flow.Identifier, *storage.StoredChunkDataPack](byChunkIDCacheSize),
		withRetrieve(retrieve),
	)

	return &ChunkDataPacks{
		db:             db,
		collections:    collections,
		byChunkIDCache: cache,
	}
}

// Store stores the given chunk data pack lists, it stores them atomically.
// Any error are exceptions
func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) error {
	batch := NewBatch(ch.db)
	defer func() {
		err := batch.Close()
		if err != nil {
			log.Error().Err(err).Msgf("failed to close batch when storing chunk data pack")
		}
	}()

	for _, c := range cs {
		err := ch.batchStore(c, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}
	}

	err := batch.Flush()
	if err != nil {
		return fmt.Errorf("cannot commit batch: %w", err)
	}

	return nil
}

// Remove removes chunk data packs by IDs, it removes them atomically.
// Any errors are exceptions
func (ch *ChunkDataPacks) Remove(cs []flow.Identifier) error {
	batch := ch.db.NewBatch()

	for _, c := range cs {
		err := ch.batchRemove(c, batch)
		if err != nil {
			return fmt.Errorf("cannot remove chunk data pack: %w", err)
		}
	}

	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("cannot commit batch: %w", err)
	}

	for _, c := range cs {
		ch.byChunkIDCache.Remove(c)
	}

	return nil
}

// ByChunkID finds the chunk data pack by chunk ID.
// it returns storage.ErrNotFound if not found
// other errors are exceptions
func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	var sc storage.StoredChunkDataPack
	err := operation.RetrieveChunkDataPack(chunkID, &sc)(ch.db)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve stored chunk data pack: %w", err)
	}

	chdp := &flow.ChunkDataPack{
		ChunkID:           sc.ChunkID,
		StartState:        sc.StartState,
		Proof:             sc.Proof,
		Collection:        nil, // to be filled in later
		ExecutionDataRoot: sc.ExecutionDataRoot,
	}
	if !sc.SystemChunk {
		collection, err := ch.collections.ByID(sc.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("could not retrive collection (id: %x) for stored chunk data pack: %w", sc.CollectionID, err)
		}

		chdp.Collection = collection
	}
	return chdp, nil
}

// BatchRemove is not used in pebble implementation
func (ch *ChunkDataPacks) BatchRemove(chunkID flow.Identifier, batch storage.BatchStorage) error {
	w := operation.NewBatchWriter(batch.GetWriter())
	return ch.batchRemove(chunkID, w)
}

func (ch *ChunkDataPacks) batchRemove(chunkID flow.Identifier, batch pebble.Writer) error {
	return operation.RemoveChunkDataPack(chunkID)(batch)
}

func (ch *ChunkDataPacks) batchStore(c *flow.ChunkDataPack, batch *Batch) error {
	sc := storage.ToStoredChunkDataPack(c)
	writer := batch.GetWriter()
	batch.OnSucceed(func() {
		ch.byChunkIDCache.Insert(sc.ChunkID, sc)
	})
	err := operation.InsertChunkDataPack(sc)(operation.NewBatchWriter(writer))
	if err != nil {
		return fmt.Errorf("failed to store chunk data pack: %w", err)
	}
	return nil
}
