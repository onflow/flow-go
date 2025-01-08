package height_based

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/block_iterator/common"
	"github.com/onflow/flow-go/storage"
)

type HeightProgressInitializer struct {
	progress storage.ConsumerProgress
	getRoot  func() (*flow.Header, error)
}

var _ module.IterateProgressInitializer = (*HeightProgressInitializer)(nil)

func NewInitializer(progress storage.ConsumerProgress, getRoot func() (*flow.Header, error)) *HeightProgressInitializer {
	return &HeightProgressInitializer{
		progress: progress,
		getRoot:  getRoot,
	}
}

func (h *HeightProgressInitializer) Init() (module.IterateProgressReader, module.IterateProgressWriter, error) {
	_, err := h.progress.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		root, err := h.getRoot()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get root block: %w", err)
		}

		next := root.Height + 1
		err = h.progress.InitProcessedIndex(next)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	nextProgress := common.NewNextProgress(h.progress)

	return nextProgress, nextProgress, nil
}
