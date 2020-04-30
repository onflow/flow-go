package integration

import "github.com/dapperlabs/flow-go/model/flow"

func FinalizedBlocks(in *Instance) []*flow.Header {
	finalized := make([]*flow.Header, 0)

	lastFinalID := in.forks.FinalizedBlock().BlockID
	finalizedBlockBlob, found := in.headers.Load(lastFinalID)
	if !found {
		return finalized
	}
	finalizedBlock := finalizedBlockBlob.(*flow.Header)

	for {
		finalized = append(finalized, finalizedBlock)
		finalizedBlockBlob, found = in.headers.Load(finalizedBlock.ParentID)
		if !found {
			break
		}
		finalizedBlock = finalizedBlockBlob.(*flow.Header)
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
