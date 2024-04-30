package counters

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/storage"
)

// PersistentStrictMonotonicCounter represents the consumer progress with strict monotonic counter.
type PersistentStrictMonotonicCounter struct {
	consumerProgress storage.ConsumerProgress

	// used to skip heights that are lower than the current height
	counter StrictMonotonousCounter
}

// NewPersistentStrictMonotonicCounter creates a new PersistentStrictMonotonicCounter which inserts the default
// processed index to the storage layer and creates new counter with defaultIndex value.
// The consumer progress and associated db entry must not be accessed outside of calls to the returned object,
// otherwise the state may become inconsistent.
//
// No errors are expected during normal operation.
func NewPersistentStrictMonotonicCounter(consumerProgress storage.ConsumerProgress, defaultIndex uint64) (*PersistentStrictMonotonicCounter, error) {
	m := &PersistentStrictMonotonicCounter{
		consumerProgress: consumerProgress,
	}

	// sync with storage for the processed index to ensure the consistency
	value, err := m.consumerProgress.ProcessedIndex()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			err := m.consumerProgress.InitProcessedIndex(defaultIndex)
			if err != nil {
				return nil, fmt.Errorf("could not init consumer progress: %w", err)
			}
			m.counter = NewMonotonousCounter(defaultIndex)
		} else {
			return nil, fmt.Errorf("could not read consumer progress: %w", err)
		}
	} else {
		m.counter = NewMonotonousCounter(value)
	}

	return m, nil
}

// Set sets the processed index, ensuring it is strictly monotonically increasing.
// Returns true if update was successful or false if stored value is larger.
func (m *PersistentStrictMonotonicCounter) Set(processed uint64) bool {
	if !m.counter.Set(processed) {
		return false
	}
	err := m.consumerProgress.SetProcessedIndex(processed)
	return err == nil
}

// Value loads the current stored index.
//
// No errors are expected during normal operation.
func (m *PersistentStrictMonotonicCounter) Value() uint64 {
	return m.counter.Value()
}
