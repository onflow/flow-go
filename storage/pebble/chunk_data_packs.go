package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

type ChunkDataPacks struct {
	db          *pebble.DB
	collections storage.Collections
}

var _ storage.ChunkDataPacks = (*ChunkDataPacks)(nil)

func NewChunkDataPacks(db *pebble.DB, collections storage.Collections) *ChunkDataPacks {
	return &ChunkDataPacks{
		db:          db,
		collections: collections,
	}
}

func (ch *ChunkDataPacks) Store(cs []*flow.ChunkDataPack) error {
	batch := ch.db.NewBatch()
	defer batch.Close()
	for _, c := range cs {
		err := ch.batchStore(c, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}
	}

	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("cannot commit batch: %w", err)
	}

	return nil
}

func (ch *ChunkDataPacks) Remove(cs []flow.Identifier) error {
	return nil
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	var sc storage.StoredChunkDataPack
	err := RetrieveChunkDataPack(ch.db, chunkID, &sc)
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

func (ch *ChunkDataPacks) BatchRemove(chunkID flow.Identifier, batch storage.BatchStorage) error {
	return nil
}

func (ch *ChunkDataPacks) batchStore(c *flow.ChunkDataPack, batch *pebble.Batch) error {
	sc := storage.ToStoredChunkDataPack(c)
	return InsertChunkDataPack(batch, sc)
}

func InsertChunkDataPack(batch *pebble.Batch, sc *storage.StoredChunkDataPack) error {
	key := makeKey(codeChunkDataPack, sc.ChunkID)
	return batchWrite(batch, key, sc)
}

func RetrieveChunkDataPack(db *pebble.DB, chunkID flow.Identifier, sc *storage.StoredChunkDataPack) error {
	key := makeKey(codeChunkDataPack, chunkID)
	return retrieve(db, key, sc)
}

func batchWrite(batch *pebble.Batch, key []byte, val interface{}) error {
	value, err := msgpack.Marshal(val)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to encode value: %w", err)
	}

	err = batch.Set(key, value, nil)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to store data: %w", err)
	}

	return nil
}

func retrieve(db *pebble.DB, key []byte, sc interface{}) error {
	val, closer, err := db.Get(key)
	if err != nil {
		return convertNotFoundError(err)
	}
	defer closer.Close()

	err = msgpack.Unmarshal(val, &sc)
	if err != nil {
		return irrecoverable.NewExceptionf("failed to decode value: %w", err)
	}
	return nil
}

const (
	codeChunkDataPack = 100
)

func makeKey(prefix byte, chunkID flow.Identifier) []byte {
	return append([]byte{prefix}, chunkID[:]...)
}
