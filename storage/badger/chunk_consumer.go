package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage/badger/operation"
)

const (
	JobConsumerChunk = "ChunkConsumer"
)

type ChunkConsumer struct {
	db *badger.DB
}

func NewChunkConsumer(db *badger.DB) *ChunkConsumer {
	return &ChunkConsumer{
		db: db,
	}
}

func (cc *ChunkConsumer) ProcessedIndex() (int, error) {
	var processed int
	err := cc.db.View(operation.RetrieveProcessedIndex(JobConsumerChunk, &processed))
	if err != nil {
		return 0, fmt.Errorf("could not retrieve processed index: %w", err)
	}
	return processed, nil
}

func (cc *ChunkConsumer) SetProcessedIndex(processed int) error {
	err := operation.RetryOnConflict(cc.db.Update, operation.SetProcessedIndex(JobConsumerChunk, processed))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}
