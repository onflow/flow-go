package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
)

var _ storage.ConsumerProgress = (*MonotonousConsumerProgress)(nil)

// MonotonousConsumerProgress represents the consumer progress with strict monotonous counter.
type MonotonousConsumerProgress struct {
	*ConsumerProgress

	// used to skip heights that are lower than the current height
	counter counters.StrictMonotonousCounter
}

// NewMonotonousConsumerProgress creates a new MonotonousConsumerProgress.
func NewMonotonousConsumerProgress(db *badger.DB, consumer string) *MonotonousConsumerProgress {
	return &MonotonousConsumerProgress{
		ConsumerProgress: NewConsumerProgress(db, consumer),
		counter:          counters.NewMonotonousCounter(0),
	}
}

// InitProcessedIndex insert the default processed index to the storage layer, can only be done once.
//
// Expected errors during normal operation:
//   - storage.ErrAlreadyExists if the key already exists in the database (initialize for the second time).
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func (m *MonotonousConsumerProgress) InitProcessedIndex(defaultIndex uint64) error {
	err := m.ConsumerProgress.InitProcessedIndex(defaultIndex)
	if err != nil {
		return err
	}
	m.counter = counters.NewMonotonousCounter(defaultIndex)

	return nil
}

// SetProcessedIndex sets the processed index, ensuring it is strictly monotonically increasing.
//
// Expected errors during normal operation:
//   - storage.ErrNotFound if the key does not already exist in the database.
//   - generic error in case of unexpected failure from the database layer or encoding failure
//     or if stored value is larger than processed.
func (m *MonotonousConsumerProgress) SetProcessedIndex(processed uint64) error {
	if !m.counter.Set(processed) {
		return fmt.Errorf("could not update to height that is lower than the current height")
	}
	return m.ConsumerProgress.SetProcessedIndex(processed)
}
