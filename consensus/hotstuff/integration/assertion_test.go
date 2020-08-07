package integration

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
)

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

func allFinalizedViews(t *testing.T, nodes []*Instance) [][]uint64 {
	allViews := make([][]uint64, 0)

	// verify all nodes arrive at the same state
	for _, node := range nodes {
		views := FinalizedViews(node)

		// finalized views ordered from small to large
		sort.Slice(views, func(i, j int) bool {
			return views[i] < views[j]
		})

		allViews = append(allViews, views)
	}

	// sort all Views by chain length
	sort.Slice(allViews, func(i, j int) bool {
		return len(allViews[i]) < len(allViews[j])
	})

	return allViews
}

func assertSafety(t *testing.T, allViews [][]uint64) {
	// find the longest chain of finalized views
	longest := allViews[len(allViews)-1]

	for _, views := range allViews {
		// each view in a chain should match with the longest chain
		for j, view := range views {
			require.Equal(t, longest[j], view, "each view in a chain must match with the view in longest chain at the same height, but didn't")
		}
	}
}

// assert all finalized views must have reached a given view to ensure enough process has been made
func assertLiveness(t *testing.T, allViews [][]uint64, view uint64) {
	// the shortest chain must made enough progress
	shortest := allViews[0]
	require.Greater(t, len(shortest), 0, "no block was finalized")
	highestView := shortest[len(shortest)-1]
	require.Greater(t, highestView, view, "did not finalize enough block")
}
