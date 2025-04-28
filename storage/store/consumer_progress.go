package store

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// ConsumerProgressInitializer is a helper to initialize the consumer progress index in storage
// It prevents the consumer from being used before initialization
type ConsumerProgressInitializer struct {
	initing  sync.Mutex
	progress *consumerProgress
}

var _ storage.ConsumerProgressInitializer = (*ConsumerProgressInitializer)(nil)

func NewConsumerProgress(db storage.DB, consumer string) *ConsumerProgressInitializer {
	progress := newConsumerProgress(db, consumer)
	return &ConsumerProgressInitializer{
		progress: progress,
	}
}

func (cpi *ConsumerProgressInitializer) Initialize(defaultIndex uint64) (storage.ConsumerProgress, error) {
	// making sure only one process is initializing at any time.
	cpi.initing.Lock()
	defer cpi.initing.Unlock()

	_, err := cpi.progress.ProcessedIndex()
	if err != nil {

		// if not initialized, then initialize with default index
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("could not retrieve processed index: %w", err)
		}

		err = cpi.progress.SetProcessedIndex(defaultIndex)
		if err != nil {
			return nil, fmt.Errorf("could not set processed index: %w", err)
		}
	}

	return cpi.progress, nil
}

type consumerProgress struct {
	db       storage.DB
	consumer string // to distinguish the consume progress between different consumers
}

var _ storage.ConsumerProgress = (*consumerProgress)(nil)

func newConsumerProgress(db storage.DB, consumer string) *consumerProgress {
	return &consumerProgress{
		db:       db,
		consumer: consumer,
	}
}

// ProcessedIndex returns the processed index for the consumer
// any error would be exception
func (cp *consumerProgress) ProcessedIndex() (uint64, error) {
	reader, err := cp.db.Reader()
	if err != nil {
		return 0, err
	}

	var processed uint64
	err = operation.RetrieveProcessedIndex(reader, cp.consumer, &processed)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve processed index: %w", err)
	}
	return processed, nil
}

// SetProcessedIndex updates the processed index for the consumer
// any error would be exception
// The caller must use ConsumerProgressInitializer to initialize the progress index in storage
func (cp *consumerProgress) SetProcessedIndex(processed uint64) error {
	err := cp.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.SetProcessedIndex(rw.Writer(), cp.consumer, processed)
	})
	if err != nil {
		return fmt.Errorf("could not update processed index: %w", err)
	}

	return nil
}
