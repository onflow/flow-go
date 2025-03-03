package block_iterator

import (
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

func NewPersistentIteratorState(initializer storage.ConsumerProgressInitializer, root uint64, latest func() (uint64, error)) (*PersistentIteratorState, error) {
	store, err := initializer.Initialize(root + 1)
	if err != nil {
		return nil, fmt.Errorf("failed to init processed index: %w", err)
	}

	return &PersistentIteratorState{
		store:  store,
		latest: latest,
	}, nil
}

func (n *PersistentIteratorState) LoadState() (uint64, error) {
	// TODO: adding cache
	return n.store.ProcessedIndex()
}

func (n *PersistentIteratorState) SaveState(next uint64) error {
	return n.store.SetProcessedIndex(next)
}

// NextRange returns the next range of blocks to iterate over
// the range is inclusive, and the end is the latest block
// if there is no block to iterate, hasNext is false
func (n *PersistentIteratorState) NextRange() (rg module.IteratorRange, hasNext bool, exception error) {
	next, err := n.LoadState()
	if err != nil {
		return module.IteratorRange{}, false, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := n.latest()
	if err != nil {
		return module.IteratorRange{}, false, fmt.Errorf("failed to get latest block: %w", err)
	}

	// if the next is the next of the latest, then there is no block to iterate
	if latest+1 == next {
		return module.IteratorRange{}, false, nil
	}

	// latest could be less than next, if the pruning-threshold was increased, which means
	// we would like to keep more data than before.
	// in this case, we should not iterate any block.
	if latest < next {
		return module.IteratorRange{}, false, nil
	}

	// iterate from next to latest (inclusive)
	return module.IteratorRange{
		Start: next,
		End:   latest,
	}, true, nil
}
