package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

type ProcessedChunk struct {
	db *badger.DB
}

func NewProcessedChunk(db *badger.DB) *ProcessedChunk {
	return &ProcessedChunk{
		db: db,
	}
}

func (h *ProcessedChunk) ProcessedIndex() (int64, error) {
	return 0, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ProcessedChunk) InitProcessedIndex() (int64, error) {
	return 0, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ProcessedChunk) SetProcessedIndex(processed int64) error {
	return fmt.Errorf("use operation.SetProcessedIndex")
}
