package committee

import (
	"github.com/onflow/flow-go/consensus/hotstuff/committee/leader"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
)

// Cluster represents the committee for a cluster of collection nodes. Cluster
// committees are epoch-scoped.
//
// Clusters build blocks on a cluster chain but must obtain identity table
// information from the main chain. Thus, block ID parameters in this Committee
// implementation reference blocks on the cluster chain, which in turn reference
// blocks on the main chain - this implementation manages that translation.
type Cluster struct {
	protoState   protocol.ReadOnlyState
	clusterState cluster.State
	me           flow.Identifier
	// pre-computed leader selection for the full lifecycle of the cluster
	selection *LeaderSelection
	// initial set of cluster committee members for this epoch, used only for
	// mapping a leader index to node ID
	// CAUTION: does not contain up-to-date weight/ejection info
	identities flow.IdentityList
}

func NewClusterCommittee(
	protoState protocol.ReadOnlyState,
	clusterState cluster.State,
	cluster protocol.Cluster,
	epoch protocol.Epoch,
	me flow.Identifier,
) {

	selection, err := leader.SelectionForCluster(nil, nil)

	com := &Cluster{
		protoState:   protoState,
		clusterState: clusterState,
		me:           me,
	}

}
