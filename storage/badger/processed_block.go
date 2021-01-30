package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

type ProcessedHeight struct {
	db *badger.DB
}

func NewProcessedHeight(db *badger.DB) *ProcessedHeight {
	return &ProcessedHeight{
		db: db,
	}
}

func (h *ProcessedHeight) ProcessedIndex() (int64, error) {
	return 0, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ProcessedHeight) InitProcessedIndex() (int64, error) {
	return 0, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ProcessedHeight) SetProcessedIndex(processed int64) error {
	return fmt.Errorf("use operation.SetProcessedIndex")
}
