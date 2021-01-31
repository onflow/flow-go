package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
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

func (cp *ConsumeProgress) ProcessedIndex() (int64, error) {
	var processed int64
	err := cp.db.View(operation.RetrieveProcessedIndex(cp.consumer, &processed))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve processed index: %w", err)
		// return 0, err
	}
	return processed, nil
}

func (cp *ConsumeProgress) InitProcessedIndex(defaultIndex int64) (bool, error) {
	err := operation.RetryOnConflict(cp.db.Update, operation.InsertProcessedIndex(cp.consumer, defaultIndex))
	// the processed index has been inited before
	if errors.Is(err, storage.ErrAlreadyExists) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("could not update processed index: %w", err)
	}

	return true, nil
}

func (cp *ConsumeProgress) SetProcessedIndex(processed int64) error {
	err := operation.RetryOnConflict(cp.db.Update, operation.SetProcessedIndex(cp.consumer, processed))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}
