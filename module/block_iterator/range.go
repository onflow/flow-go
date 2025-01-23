package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/module"
)

type IteratorRangeCreator struct {
	latest func() (uint64, error)
}

var _ module.IteratorRangeCreator = (*IteratorRangeCreator)(nil)

func NewIteratorRangeCreator(latest func() (uint64, error)) *IteratorRangeCreator {
	return &IteratorRangeCreator{
		latest: latest,
	}
}

func (h *IteratorRangeCreator) CreateRange(reader module.IterateProgressReader) (module.IterateRange, error) {
	next, err := reader.LoadState()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := h.latest()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	// iterate from next to latest (inclusive)
	return module.IterateRange{
		Start: next,
		End:   latest,
	}, nil
}
