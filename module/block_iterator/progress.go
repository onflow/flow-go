package block_iterator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type NextProgress struct {
	store  storage.ConsumerProgress
	latest func() (uint64, error)
}

var _ module.IterateProgress = (*NextProgress)(nil)

func NewNextProgress(store storage.ConsumerProgress, root uint64, latest func() (uint64, error)) (*NextProgress, error) {
	_, err := store.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		next := root + 1
		err = store.InitProcessedIndex(next)
		if err != nil {
			return nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	return &NextProgress{
		store:  store,
		latest: latest,
	}, nil
}

func (n *NextProgress) LoadState() (uint64, error) {
	return n.store.ProcessedIndex()
}

func (n *NextProgress) SaveState(next uint64) error {
	return n.store.SetProcessedIndex(next)
}

// NextRange returns the next range of blocks to iterate over
func (n *NextProgress) NextRange() (module.IterateRange, error) {
	next, err := n.LoadState()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := n.latest()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	if latest < next {
		return module.IterateRange{}, fmt.Errorf("latest block is less than next block: %d < %d", latest, next)
	}

	// iterate from next to latest (inclusive)
	return module.IterateRange{
		Start: next,
		End:   latest,
	}, nil
}
