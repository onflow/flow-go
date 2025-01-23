package block_iterator

import (
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type NextProgress struct {
	store storage.ConsumerProgress
}

var _ module.IterateProgressReader = (*NextProgress)(nil)
var _ module.IterateProgressWriter = (*NextProgress)(nil)

func NewNextProgress(store storage.ConsumerProgress) *NextProgress {
	return &NextProgress{
		store: store,
	}
}

func (n *NextProgress) LoadState() (uint64, error) {
	return n.store.ProcessedIndex()
}

func (n *NextProgress) SaveState(next uint64) error {
	return n.store.SetProcessedIndex(next)
}
