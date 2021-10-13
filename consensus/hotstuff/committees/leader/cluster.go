package leader

import (
	"fmt"

	"github.com/onflow/flow-go/model/indices"
	"github.com/onflow/flow-go/state/protocol"
)

// SelectionForCluster pre-computes and returns leaders for the given cluster
// committee in the given epoch.
func SelectionForCluster(cluster protocol.Cluster, epoch protocol.Epoch) (*LeaderSelection, error) {

	// sanity check to ensure the cluster and epoch match
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	if counter != cluster.EpochCounter() {
		return nil, fmt.Errorf("inconsistent counter between epoch (%d) and cluster (%d)", counter, cluster.EpochCounter())
	}

	identities := cluster.Members()
	seed, err := epoch.Seed(indices.ProtocolCollectorClusterLeaderSelection(cluster.Index())...)
	if err != nil {
		return nil, fmt.Errorf("could not get leader selection seed for cluster (index: %v) at epoch: %v: %w", cluster.Index(), counter, err)
	}
	firstView := cluster.RootBlock().Header.View
	// TODO what is a good value here?
	finalView := firstView + EstimatedSixMonthOfViews
	leaders, err := ComputeLeaderSelectionFromSeed(
		firstView,
		seed,
		int(finalView-firstView+1),
		identities,
	)
	return leaders, err
}
