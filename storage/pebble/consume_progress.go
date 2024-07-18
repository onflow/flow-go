package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage/pebble/operation"
)

type ConsumerProgress struct {
	db       *pebble.DB
	consumer string // to distinguish the consume progress between different consumers
}

func NewConsumerProgress(db *pebble.DB, consumer string) *ConsumerProgress {
	return &ConsumerProgress{
		db:       db,
		consumer: consumer,
	}
}

func (cp *ConsumerProgress) ProcessedIndex() (uint64, error) {
	var processed uint64
	err := operation.RetrieveProcessedIndex(cp.consumer, &processed)(cp.db)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve processed index: %w", err)
	}
	return processed, nil
}

// InitProcessedIndex insert the default processed index to the storage layer, can only be done once.
// initialize for the second time will return storage.ErrAlreadyExists
func (cp *ConsumerProgress) InitProcessedIndex(defaultIndex uint64) error {
	err := operation.InsertProcessedIndex(cp.consumer, defaultIndex)(cp.db)
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

func (cp *ConsumerProgress) SetProcessedIndex(processed uint64) error {
	err := operation.SetProcessedIndex(cp.consumer, processed)(cp.db)
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}
