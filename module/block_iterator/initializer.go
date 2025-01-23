package block_iterator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type ProgressInitializer struct {
	progress storage.ConsumerProgress
	root     uint64
}

var _ module.IterateProgressInitializer = (*ProgressInitializer)(nil)

func NewInitializer(progress storage.ConsumerProgress, root uint64) *ProgressInitializer {
	return &ProgressInitializer{
		progress: progress,
		root:     root,
	}
}

func (h *ProgressInitializer) Init() (module.IterateProgressReader, module.IterateProgressWriter, error) {
	_, err := h.progress.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		next := h.root + 1
		err = h.progress.InitProcessedIndex(next)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	nextProgress := NewNextProgress(h.progress)

	return nextProgress, nextProgress, nil
}
