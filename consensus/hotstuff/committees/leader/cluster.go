package leader

import (
	"fmt"

	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/prg"
)

// SelectionForCluster pre-computes and returns leaders for the given cluster
// committee in the given epoch. A cluster containing nodes with zero `InitialWeight`
// is an accepted input as long as there are nodes with positive weights. Zero-weight nodes
// have zero probability of being selected as leaders in accordance with their weight.
func SelectionForCluster(cluster protocol.Cluster, epoch protocol.CommittedEpoch) (*LeaderSelection, error) {
	// sanity check to ensure the cluster and epoch match
	counter := epoch.Counter()
	if counter != cluster.EpochCounter() {
		return nil, fmt.Errorf("inconsistent counter between epoch (%d) and cluster (%d)", counter, cluster.EpochCounter())
	}

	// get the random source of the current epoch
	randomSeed := epoch.RandomSource()
	// create random number generator from the seed and customizer
	rng, err := prg.New(randomSeed, prg.CollectorClusterLeaderSelection(cluster.Index()), nil)
	if err != nil {
		return nil, fmt.Errorf("could not create rng: %w", err)
	}

	firstView := cluster.RootBlock().View
	epochFirstView := epoch.FirstView()
	epochFinalView := epoch.FinalView()
	// Use the epoch's view range plus a 2-epoch buffer for leader selection.
	// Clusters only exist for the duration of their epoch, so pre-computing
	// leader selection far beyond the epoch's final view is wasteful.
	// The 2-epoch buffer provides safety margin for potential epoch extensions
	// during Epoch Fallback Mode (EFM), while avoiding the massive memory overhead
	// of the previous 6-month (EstimatedSixMonthOfViews) pre-computation.
	epochLength := epochFinalView - epochFirstView
	finalView := epochFinalView + 2*epochLength

	// ComputeLeaderSelection already handles zero-weight nodes with marginal overhead.
	leaders, err := ComputeLeaderSelection(
		firstView,
		rng,
		int(finalView-firstView+1),
		cluster.Members().ToSkeleton(),
	)
	return leaders, err
}
