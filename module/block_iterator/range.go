package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/module"
)

func CreateRange(reader module.IterateProgressReader, getLatest func() (uint64, error)) (module.IterateRange, error) {
	next, err := reader.LoadState()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to read next height: %w", err)
	}

	latest, err := getLatest()
	if err != nil {
		return module.IterateRange{}, fmt.Errorf("failed to get latest block: %w", err)
	}

	// iterate from next to latest (inclusive)
	return module.IterateRange{
		Start: next,
		End:   latest,
	}, nil
}
