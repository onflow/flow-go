package persistent_strict_counters

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
)

// PersistentStrictMonotonicCounter represents the consumer progress with strict monotonic counter.
type PersistentStrictMonotonicCounter struct {
	consumerProgress *bstorage.ConsumerProgress

	// used to skip heights that are lower than the current height
	counter counters.StrictMonotonousCounter
}

// NewPersistentStrictMonotonicCounter creates a new PersistentStrictMonotonicCounter which inserts the default
// processed index to the storage layer and creates new counter with defaultIndex value.
// The consumer progress and associated db entry must not be accessed outside of calls to the returned object,
// otherwise the state may become inconsistent.
//
// No errors are expected during normal operation.
func NewPersistentStrictMonotonicCounter(db *badger.DB, consumer string, defaultIndex uint64) (*PersistentStrictMonotonicCounter, error) {
	m := &PersistentStrictMonotonicCounter{
		consumerProgress: bstorage.NewConsumerProgress(db, consumer),
	}

	// sync with storage for the processed index to ensure the consistency
	value, err := m.consumerProgress.ProcessedIndex()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err := m.consumerProgress.InitProcessedIndex(defaultIndex)
			if err != nil {
				return nil, fmt.Errorf("could not init %s consumer progress: %w", consumer, err)
			}
			m.counter = counters.NewMonotonousCounter(defaultIndex)
		} else {
			return nil, fmt.Errorf("could not read %s consumer progress: %w", consumer, err)
		}
	} else {
		m.counter = counters.NewMonotonousCounter(value)
	}

	return m, nil
}

// Set sets the processed index, ensuring it is strictly monotonically increasing.
//
// Expected errors during normal operation:
//   - generic error in case of unexpected failure from the database layer or encoding failure
//     or if stored value is larger than processed.
func (m *PersistentStrictMonotonicCounter) Set(processed uint64) error {
	if !m.counter.Set(processed) {
		return fmt.Errorf("could not update to height that is lower than the current height")
	}
	return m.consumerProgress.SetProcessedIndex(processed)
}

// Value loads the current stored index.
//
// No errors are expected during normal operation.
func (m *PersistentStrictMonotonicCounter) Value() uint64 {
	return m.counter.Value()
}
