package block_iterator

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

func CreateIndexedBlockIterator(
	getBlockIDByIndex func(uint64) (flow.Identifier, error),
	progress storage.ConsumerProgress,
	getRoot func() (uint64, error),
	latest func() (uint64, error),
) (module.BlockIterator, error) {

	initializer := NewInitializer(progress, getRoot)
	progressReader, progressWriter, err := initializer.Init()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize progress: %w", err)
	}

	rangeCreator := NewIteratorRangeCreator(latest)
	iterRange, err := rangeCreator.CreateRange(progressReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create range for block iteration: %w", err)
	}

	return NewIndexedBlockIterator(getBlockIDByIndex, progressWriter, iterRange), nil
}
