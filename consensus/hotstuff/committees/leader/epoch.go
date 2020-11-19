package leader

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/indices"
	"github.com/onflow/flow-go/state/protocol"
)

// SelectionForEpoch pre-computes and returns leaders for the consensus committee
// in the given epoch.
// TODO: this should replace the consensus.go in this package
func SelectionForEpoch(epoch protocol.Epoch) (*committees.LeaderSelection, error) {

	// pre-compute leader selection for the epoch
	identities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch initial identities: %w", err)
	}
	seed, err := epoch.Seed(indices.ProtocolConsensusLeaderSelection...)
	if err != nil {
		return nil, fmt.Errorf("could not get epoch seed: %w", err)
	}
	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch final view: %w", err)
	}
	leaders, err := committees.ComputeLeaderSelectionFromSeed(
		firstView,
		seed,
		int(finalView-firstView+1), // add 1 because both first/final view are inclusive
		identities.Filter(filter.HasRole(flow.RoleConsensus)),
	)
	return leaders, err
}

// SelectionForCluster pre-computes and returns leaders for the given cluster
// committee in the given epoch.
func SelectionForCluster(cluster protocol.Cluster, epoch protocol.Epoch) (*committees.LeaderSelection, error) {

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
	}
	firstView := cluster.RootBlock().Header.View
	// TODO what is a good value here?
	finalView := firstView + EstimatedSixMonthOfViews
	leaders, err := committees.ComputeLeaderSelectionFromSeed(
		firstView,
		seed,
		int(finalView-firstView+1),
		identities,
	)
	return leaders, err
}
