package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

type ConsumeProgress struct {
	db       *badger.DB
	consumer string // to distinguish the consume progress between different consumers
}

func NewConsumeProgress(db *badger.DB, consumer string) *ConsumeProgress {
	return &ConsumeProgress{
		db:       db,
		consumer: consumer,
	}
}

func (h *ConsumeProgress) ProcessedIndex() (int64, error) {
	return 0, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ConsumeProgress) InitProcessedIndex(initIndex int64) (bool, error) {
	return false, fmt.Errorf("use operation.SetProcessedIndex")
}

func (h *ConsumeProgress) SetProcessedIndex(processed int64) error {
	return fmt.Errorf("use operation.SetProcessedIndex")
}
