package leader

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/indices"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// NewSelectionForCollection returns a leader selection instances that caches the leader selection for collection cluster.
// count: the number of views to pre-generate the leader selection for and cache
// rootHeader: the root block to query the identities from.
// rootQC: the QC contains the random beacon, which determines the random seed.
// clusterRootHeader, clusterState: to determine the cluster index, which determines an indices to seed the random generator
func NewSelectionForCollection(count int, rootHeader *flow.Header, rootQC *model.QuorumCertificate, state protocol.State, clusterRootHeader *flow.Header, clusterState cluster.State, clusterIndex uint) (*committee.LeaderSelection, error) {
	// random indices is used for deriving random seed, which is used by the leader selection algorithm
	// each collection cluster will have different random indices, therefore have different random seed.
	inds := indices.ProtocolCollectorClusterLeaderSelection(uint32(clusterIndex))

	seed, err := ReadSeed(inds, rootHeader, rootQC, state)
	if err != nil {
		return nil, fmt.Errorf("could not read seed: %w", err)
	}

	// always use the root header's snapshot to find the identities and their stakes
	clusters, err := state.AtBlockID(rootHeader.ID()).Clusters()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}

	// find all collection nodes identities which contain the stake info
	cluster, found := clusters.ByIndex(clusterIndex)
	if !found {
		return nil, fmt.Errorf("could not find cluster (index: %d)", clusterIndex)
	}

	selection, err := committee.ComputeLeaderSelectionFromSeed(clusterRootHeader.View, seed, count, cluster)
	if err != nil {
		return nil, fmt.Errorf("could not compute leader selection from seed: %w", err)
	}

	return selection, nil
}
