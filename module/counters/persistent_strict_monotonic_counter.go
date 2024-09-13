package counters

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/storage"
)

// ErrIncorrectValue indicates that a processed value is lower or equal than current value.
var (
	ErrIncorrectValue = errors.New("incorrect value")
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
func NewPersistentStrictMonotonicCounter(factory storage.ConsumerProgressFactory, defaultIndex uint64) (*PersistentStrictMonotonicCounter, error) {
	consumerProgress, err := factory.InitConsumer(defaultIndex)
	if err != nil {
		return nil, fmt.Errorf("could not init consumer progress: %w", err)
	}

	m := &PersistentStrictMonotonicCounter{
		consumerProgress: consumerProgress,
	}

	// sync with storage for the processed index to ensure the consistency
	value, err := m.consumerProgress.ProcessedIndex()
	if err != nil {
		return nil, fmt.Errorf("could not read consumer progress: %w", err)
	}

	m.counter = NewMonotonousCounter(value)

	return m, nil
}

// Set sets the processed index, ensuring it is strictly monotonically increasing.
//
// Expected errors during normal operation:
//   - codes.ErrIncorrectValue - if stored value is larger than processed.
//   - generic error in case of unexpected failure from the database layer or
//     encoding failure.
func (m *PersistentStrictMonotonicCounter) Set(processed uint64) error {
	if !m.counter.Set(processed) {
		return ErrIncorrectValue
	}
	return m.consumerProgress.SetProcessedIndex(processed)
}

// Value loads the current stored index.
//
// No errors are expected during normal operation.
func (m *PersistentStrictMonotonicCounter) Value() uint64 {
	return m.counter.Value()
}
