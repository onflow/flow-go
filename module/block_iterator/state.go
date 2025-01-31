package block_iterator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// PersistentIteratorState stores the state of the iterator in a persistent storage
type PersistentIteratorState struct {
	store  storage.ConsumerProgress
	latest func() (uint64, error)
}

var _ module.IteratorState = (*PersistentIteratorState)(nil)

func NewPersistentIteratorState(store storage.ConsumerProgress, root uint64, latest func() (uint64, error)) (*PersistentIteratorState, error) {
	_, err := store.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		next := root + 1
		err = store.InitProcessedIndex(next)
		if err != nil {
			return nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	return &PersistentIteratorState{
		store:  store,
		latest: latest,
	}, nil
}

func (n *PersistentIteratorState) LoadState() (uint64, error) {
	return n.store.ProcessedIndex()
}

func (n *PersistentIteratorState) SaveState(next uint64) error {
	return n.store.SetProcessedIndex(next)
}

// NextRange returns the next range of blocks to iterate over
func (n *PersistentIteratorState) NextRange() (module.IteratorRange, error) {
	next, err := n.LoadState()
	if err != nil {
		return module.IteratorRange{}, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := n.latest()
	if err != nil {
		return module.IteratorRange{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	if latest < next {
		return module.IteratorRange{}, fmt.Errorf("latest block is less than next block: %d < %d", latest, next)
	}

	// iterate from next to latest (inclusive)
	return module.IteratorRange{
		Start: next,
		End:   latest,
	}, nil
}
