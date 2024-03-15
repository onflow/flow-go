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
func SelectionForCluster(cluster protocol.Cluster, epoch protocol.Epoch) (*LeaderSelection, error) {
	// sanity check to ensure the cluster and epoch match
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	if counter != cluster.EpochCounter() {
		return nil, fmt.Errorf("inconsistent counter between epoch (%d) and cluster (%d)", counter, cluster.EpochCounter())
	}

	// get the random source of the current epoch
	randomSeed, err := epoch.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get leader selection seed for cluster (index: %v) at epoch: %v: %w", cluster.Index(), counter, err)
	}
	// create random number generator from the seed and customizer
	rng, err := prg.New(randomSeed, prg.CollectorClusterLeaderSelection(cluster.Index()), nil)
	if err != nil {
		return nil, fmt.Errorf("could not create rng: %w", err)
	}

	firstView := cluster.RootBlock().Header.View
	finalView := firstView + EstimatedSixMonthOfViews // TODO what is a good value here?

	// ComputeLeaderSelection already handles zero-weight nodes with marginal overhead.
	leaders, err := ComputeLeaderSelection(
		firstView,
		rng,
		int(finalView-firstView+1),
		cluster.Members().ToSkeleton(),
	)
	return leaders, err
}
