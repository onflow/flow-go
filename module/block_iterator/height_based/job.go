package height_based

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type HeightIteratorJobCreator struct {
	latest func() (*flow.Header, error)
}

var _ module.IteratorJobCreator = (*HeightIteratorJobCreator)(nil)

func NewHeightIteratorJobCreator(latest func() (*flow.Header, error)) *HeightIteratorJobCreator {
	return &HeightIteratorJobCreator{
		latest: latest,
	}
}

func (h *HeightIteratorJobCreator) CreateJob(reader module.IterateProgressReader) (module.IterateJob, error) {
	next, err := reader.ReadNext()
	if err != nil {
		return module.IterateJob{}, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := h.latest()
	if err != nil {
		return module.IterateJob{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	// iterate from next to latest (inclusive)
	return module.IterateJob{
		Start: next,
		End:   latest.Height,
	}, nil
}
