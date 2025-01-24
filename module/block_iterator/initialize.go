package block_iterator

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func InitializeProgress(progress storage.ConsumerProgress, root uint64) (module.IterateProgress, error) {
	_, err := progress.ProcessedIndex()
	if errors.Is(err, storage.ErrNotFound) {
		next := root + 1
		err = progress.InitProcessedIndex(next)
		if err != nil {
			return nil, fmt.Errorf("failed to init processed index: %w", err)
		}
	}

	nextProgress := NewNextProgress(progress)

	return nextProgress, nil
}
