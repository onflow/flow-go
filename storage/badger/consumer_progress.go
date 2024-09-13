package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// ConsumerProgressFactory is a factory to create consumer progress
// It ensures the consumer progress is initialized before being used
type ConsumerProgressFactory struct {
	db       *badger.DB
	consumer string
}

var _ storage.ConsumerProgressFactory = (*ConsumerProgressFactory)(nil)

func NewConsumerProgressFactory(db *badger.DB, consumer string) *ConsumerProgressFactory {
	return &ConsumerProgressFactory{
		db:       db,
		consumer: consumer,
	}
}

// InitConsumer inserts the default processed index to the storage layer to initialize the
// consumer if it has not been initialized before, and returns the initalized consumer progress.
// It should not be called concurrently
func (cpf *ConsumerProgressFactory) InitConsumer(defaultIndex uint64) (storage.ConsumerProgress, error) {
	consumer := newConsumerProgress(cpf.db, cpf.consumer)

	_, err := consumer.ProcessedIndex()
	if err == nil {
		// the consumer progress factory has been initialized
		return consumer, nil
	}

	if !storage.IsNotFound(err) {
		return nil, fmt.Errorf("could not check if consumer progress is initted for consumer %v: %w",
			consumer, err)
	}

	// never initialized, initialize now
	err = operation.WithBatchWriter(cpf.db, operation.SetProcessedIndex(cpf.consumer, defaultIndex))
	if err != nil {
		return nil, fmt.Errorf("could not init consumer progress for consumer %v: %w", cpf.consumer, err)
	}

	return consumer, nil
}

type ConsumerProgress struct {
	db       *badger.DB
	consumer string // to distinguish the consume progress between different consumers
}

var _ storage.ConsumerProgress = (*ConsumerProgress)(nil)

func newConsumerProgress(db *badger.DB, consumer string) *ConsumerProgress {
	return &ConsumerProgress{
		db:       db,
		consumer: consumer,
	}
}

func (cp *ConsumerProgress) ProcessedIndex() (uint64, error) {
	var processed uint64
	err := operation.RetrieveProcessedIndex(cp.consumer, &processed)(operation.ToReader(cp.db))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve processed index: %w", err)
	}
	return processed, nil
}

func (cp *ConsumerProgress) SetProcessedIndex(processed uint64) error {
	err := operation.WithBatchWriter(cp.db, operation.SetProcessedIndex(cp.consumer, processed))
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}
