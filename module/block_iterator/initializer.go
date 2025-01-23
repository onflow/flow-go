package block_iterator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type ProgressInitializer struct {
	progress storage.ConsumerProgress
	getRoot  func() (uint64, error)
}

var _ module.IterateProgressInitializer = (*ProgressInitializer)(nil)

func NewInitializer(progress storage.ConsumerProgress, getRoot func() (uint64, error)) *ProgressInitializer {
	return &ProgressInitializer{
		progress: progress,
		getRoot:  getRoot,
	}
}

func (h *ProgressInitializer) Init() (module.IterateProgressReader, module.IterateProgressWriter, error) {
	_, err := h.progress.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		root, err := h.getRoot()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get root block: %w", err)
		}

		next := root + 1
		err = h.progress.InitProcessedIndex(next)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	nextProgress := NewNextProgress(h.progress)

	return nextProgress, nextProgress, nil
}
