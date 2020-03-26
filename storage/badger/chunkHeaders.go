package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

type ChunkHeaders struct {
	db *badger.DB
}

func NewChunkHeaders(db *badger.DB) *ChunkHeaders {
	ch := ChunkHeaders{
		db: db,
	}
	return &ch
}

func (ch *ChunkHeaders) Store(c *flow.ChunkHeader) error {
	return ch.db.Update(func(btx *badger.Txn) error {
		err := operation.InsertChunkHeader(c)(btx)
		if err != nil {
			return fmt.Errorf("could not insert chunk header: %w", err)
		}
		return nil
	})
}

func (ch *ChunkHeaders) Remove(chunkID flow.Identifier) error {
	return ch.db.Update(func(btx *badger.Txn) error {
		err := operation.RemoveChunkHeader(chunkID)(btx)
		if err != nil {
			return fmt.Errorf("could not remove chunk header: %w", err)
		}
		return nil
	})
}

func (ch *ChunkHeaders) ByID(chunkID flow.Identifier) (*flow.ChunkHeader, error) {
	var c flow.ChunkHeader
	err := ch.db.View(func(btx *badger.Txn) error {
		err := operation.RetrieveChunkHeader(chunkID, &c)(btx)
		if err != nil {
			return fmt.Errorf("could not retrieve chunk header: %w", err)
		}
		return nil
	})

	return &c, err
}
