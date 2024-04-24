package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/counters"
)

// MonotonicConsumerProgress represents the consumer progress with strict monotonic counter.
type MonotonicConsumerProgress struct {
	consumerProgress *ConsumerProgress

	// used to skip heights that are lower than the current height
	counter counters.StrictMonotonousCounter
}

// NewMonotonicConsumerProgress creates a new MonotonicConsumerProgress which inserts the default
// processed index to the storage layer and creates new counter with defaultIndex value.
//
// No errors are expected during normal operation.
func NewMonotonicConsumerProgress(db *badger.DB, consumer string, defaultIndex uint64) (*MonotonicConsumerProgress, error) {
	m := &MonotonicConsumerProgress{
		consumerProgress: NewConsumerProgress(db, consumer),
		counter:          counters.NewMonotonousCounter(defaultIndex),
	}

	err := m.consumerProgress.InitProcessedIndex(defaultIndex)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Store sets the processed index, ensuring it is strictly monotonically increasing.
//
// Expected errors during normal operation:
//   - generic error in case of unexpected failure from the database layer or encoding failure
//     or if stored value is larger than processed.
func (m *MonotonicConsumerProgress) Store(processed uint64) error {
	if !m.counter.Set(processed) {
		return fmt.Errorf("could not update to height that is lower than the current height")
	}
	return m.consumerProgress.SetProcessedIndex(processed)
}

// Load loads the current processed index.
//
// No errors are expected during normal operation.
func (m *MonotonicConsumerProgress) Load() (uint64, error) {
	return m.consumerProgress.ProcessedIndex()
}
