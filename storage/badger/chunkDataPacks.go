package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
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
	return ch.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertChunkDataPack(c)(btx)
		if err != nil {
			return fmt.Errorf("could not insert chunk data pack: %w", err)
		}
		return nil
	})
}

func (ch *ChunkDataPacks) Remove(chunkID flow.Identifier) error {
	return ch.db.Update(func(btx *badger.Txn) error {
		err := operation.RemoveChunkDataPack(chunkID)(btx)
		if err != nil {
			return fmt.Errorf("could not remove chunk data pack: %w", err)
		}
		return nil
	})
}

func (ch *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	var c flow.ChunkDataPack
	err := ch.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveChunkDataPack(chunkID, &c)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve chunk data pack: %w", err)
		}
		return nil
	})

	return &c, err
}
