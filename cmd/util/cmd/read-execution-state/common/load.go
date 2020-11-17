package common

import (
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/module/metrics"
)

func LoadExecutionState(executionDir string) (*mtrie.Forest, error) {
	w, err := wal.NewWAL(
		nil,
		nil,
		executionDir,
		complete.DefaultCacheSize,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	if err != nil {
		return nil, err
	}

	forest, err := mtrie.NewForest(pathfinder.PathByteSize, executionDir, complete.DefaultCacheSize, metrics.NewNoopCollector(), nil)
	if err != nil {
		return nil, err
	}

	err = w.ReplayOnForest(forest)
	if err != nil {
		return nil, err
	}

	return forest, nil
}
