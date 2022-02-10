package integration

import "github.com/onflow/flow-go/model/flow"

func FinalizedBlocks(in *Instance) []*flow.Header {
	finalized := make([]*flow.Header, 0)

	lastFinalID := in.forks.FinalizedBlock().BlockID
	in.updatingBlocks.RLock()
	finalizedBlock, found := in.headers[lastFinalID]
	defer in.updatingBlocks.RUnlock()
	if !found {
		return finalized
	}

	for {
		finalized = append(finalized, finalizedBlock)
		finalizedBlock, found = in.headers[finalizedBlock.ParentID]
		if !found {
			break
		}
	}
	return finalized
}

func FinalizedViews(in *Instance) []uint64 {
	finalizedBlocks := FinalizedBlocks(in)
	views := make([]uint64, 0, len(finalizedBlocks))
	for _, b := range finalizedBlocks {
		views = append(views, b.View)
	}
	return views
}
