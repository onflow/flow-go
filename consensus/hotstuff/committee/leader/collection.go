package leader

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/indices"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
)

func NewSelectionForCollection(count int, rootHeader *flow.Header, rootQC *model.QuorumCertificate, st protocol.State, clusterRootHeader *flow.Header, clusterState cluster.State, clusterIndex uint) (*committee.LeaderSelection, error) {
	// random indices is used for deriving random seed, which is used by the leader selection algorithm
	// each collection cluster will have different random indices, therefore have different random seed.
	inds := indices.ProtocolCollectorClusterLeaderSelection(uint32(clusterIndex))

	seed, err := ReadSeed(inds, rootHeader, rootQC, st)
	if err != nil {
		return nil, fmt.Errorf("could not read seed: %w", err)
	}

	// find all collection nodes identities which contain the stake info
	identities, err := st.AtBlockID(rootHeader.ID()).Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get consensus identities: %w", err)
	}

	selection, err := committee.ComputeLeaderSelectionFromSeed(clusterRootHeader.View, seed, count, identities)
	if err != nil {
		return nil, fmt.Errorf("could not compute leader selection from seed: %w", err)
	}

	return selection, nil
}
