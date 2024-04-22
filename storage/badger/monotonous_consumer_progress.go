package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/storage"
)

var _ storage.ConsumerProgress = (*MonotonousConsumerProgress)(nil)

type MonotonousConsumerProgress struct {
	*ConsumerProgress

	// used to skip heights that are lower than the current height
	counter counters.StrictMonotonousCounter
}

func NewMonotonousConsumerProgress(db *badger.DB, consumer string) *MonotonousConsumerProgress {
	return &MonotonousConsumerProgress{
		ConsumerProgress: NewConsumerProgress(db, consumer),
		counter:          counters.NewMonotonousCounter(0),
	}
}

// InitProcessedIndex insert the default processed index to the storage layer, can only be done once.
// initialize for the second time will return storage.ErrAlreadyExists
func (m *MonotonousConsumerProgress) InitProcessedIndex(defaultIndex uint64) error {
	err := m.ConsumerProgress.InitProcessedIndex(defaultIndex)
	if err != nil {
		return err
	}
	m.counter = counters.NewMonotonousCounter(defaultIndex)

	return nil
}

func (m *MonotonousConsumerProgress) SetProcessedIndex(processed uint64) error {
	if !m.counter.Set(processed) {
		return fmt.Errorf("could not update to height that is lower than the current height")
	}
	return m.ConsumerProgress.SetProcessedIndex(processed)
}

func (m *MonotonousConsumerProgress) ProcessedIndex() (uint64, error) {
	return m.ConsumerProgress.ProcessedIndex()
}
