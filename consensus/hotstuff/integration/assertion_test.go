package integration

import "github.com/dapperlabs/flow-go/model/flow"

func FinalizedBlocks(in *Instance) []*flow.Header {
	finalized := make([]*flow.Header, 0)

	lastFinalID := in.forks.FinalizedBlock().BlockID
	fin, found := in.headers[lastFinalID]
	if !found {
		return finalized
	}

	for fin != nil {
		finalized = append(finalized, fin)
		fin, _ = in.headers[fin.ParentID]
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
