package pruner

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/block_iterator/latest"
)

// LatestPrunable decides which blocks are prunable
// we don't want to prune all the sealed blocks, but keep
// a certain number of them so that the data is still available for querying
type LatestPrunable struct {
	*latest.LatestSealedAndExecuted
	threshold uint64 // the number of blocks below the latest block
}

func (l *LatestPrunable) Latest() (*flow.Header, error) {
	return l.LatestSealedAndExecuted.BelowLatest(l.threshold)
}
