package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/operation"
)

type ConsumerProgress struct {
	db       *badger.DB
	consumer string // to distinguish the consume progress between different consumers
}

func NewConsumerProgress(db *badger.DB, consumer string) *ConsumerProgress {
	return &ConsumerProgress{
		db:       db,
		consumer: consumer,
	}
}

func (cp *ConsumerProgress) ProcessedIndex() (uint64, error) {
	var processed uint64
	err := cp.db.View(operation.RetrieveProcessedIndex(cp.consumer, &processed))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve processed index: %w", err)
	}
	return processed, nil
}

// InitProcessedIndex insert the default processed index to the storage layer, can only be done once.
// initialize for the second time will return storage.ErrAlreadyExists
func (cp *ConsumerProgress) InitProcessedIndex(defaultIndex uint64) error {
	err := operation.RetryOnConflict(cp.db.Update, operation.InsertProcessedIndex(cp.consumer, defaultIndex))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

func (cp *ConsumerProgress) SetProcessedIndex(processed uint64) error {
	err := operation.RetryOnConflict(cp.db.Update, operation.SetProcessedIndex(cp.consumer, processed))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

func (cp *ConsumerProgress) Halted() (bool, error) {
	var halted bool
	err := cp.db.View(operation.RetrieveHalted(cp.consumer, &halted))
	if err != nil {
		return false, fmt.Errorf("failed to retrieve halted status: %w", err)
	}
	return halted, nil
}

// InitHalted insert unhalted status to the storage layer, can only be done once.
// initialize for the second time will return storage.ErrAlreadyExists
func (cp *ConsumerProgress) InitHalted() error {
	err := operation.RetryOnConflict(cp.db.Update, operation.InsertHalted(cp.consumer, false))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}

func (cp *ConsumerProgress) SetHalted(halted bool) error {
	err := operation.RetryOnConflict(cp.db.Update, operation.SetHalted(cp.consumer, halted))
	if err != nil {
		return fmt.Errorf("could not update halted status: %w", err)
	}

	return nil
}
