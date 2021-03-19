package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type ChunkDataPacks struct {
	db *badger.DB
}

func NewChunkDataPacks(db *badger.DB) *ChunkDataPacks {
	ch := ChunkDataPacks{
		db: db,
	}
	return &ch
}

func (ch *ChunkDataPacks) Store(c *flow.ChunkDataPack) error {
	err := operation.RetryOnConflict(ch.db.Update, operation.SkipDuplicates(operation.InsertChunkDataPack(c)))
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
	return nil
}

func (ch *ChunkDataPacks) BatchStore(c *flow.ChunkDataPack, batch storage.BatchStorage) error {
	if writeBatch, ok := batch.(*badger.WriteBatch); ok {
		return operation.BatchInsertChunkDataPack(c)(writeBatch)
	}
	return fmt.Errorf("unsupported BatchStore type %T", batch)
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	var c flow.ChunkDataPack
	err := ch.db.View(operation.RetrieveChunkDataPack(chunkID, &c))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve chunk datapack: %w", err)
	}
	return &c, nil
}
