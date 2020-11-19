package committee

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committee/leader"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Cluster represents the committee for a cluster of collection nodes. Cluster
// committees are epoch-scoped.
//
// Clusters build blocks on a cluster chain but must obtain identity table
// information from the main chain. Thus, block ID parameters in this Committee
// implementation reference blocks on the cluster chain, which in turn reference
// blocks on the main chain - this implementation manages that translation.
type Cluster struct {
	state    protocol.ReadOnlyState
	payloads storage.ClusterPayloads
	me       flow.Identifier
	// pre-computed leader selection for the full lifecycle of the cluster
	selection *LeaderSelection
	// initial set of cluster committee members for this epoch, used only for
	// mapping a leader index to node ID
	// CAUTION: does not contain up-to-date weight/ejection info
	identities flow.IdentityList
}

func NewClusterCommittee(
	state protocol.ReadOnlyState,
	payloads storage.ClusterPayloads,
	cluster protocol.Cluster,
	epoch protocol.Epoch,
	me flow.Identifier,
) (*Cluster, error) {

	selection, err := leader.SelectionForCluster(cluster, epoch)
	if err != nil {
		return nil, fmt.Errorf("could not compute leader selection for cluster: %w", err)
	}

	com := &Cluster{
		state:      state,
		payloads:   payloads,
		me:         me,
		selection:  selection,
		identities: cluster.Members(),
	}
	return com, nil
}

func (c *Cluster) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {

	// first retrieve the cluster block payload
	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster payload: %w", err)
	}

	// if a reference block is specified, use that
	if payload.ReferenceBlockID != flow.ZeroID {
		identities, err := c.state.AtBlockID(payload.ReferenceBlockID).Identities(filter.And(
			selector,
			c.identities.Selector(),
		))
		return identities, err
	}

	// otherwise, this is a root block, in which case we use the initial cluster members
	return c.identities.Filter(selector), nil
}

func (c *Cluster) Identity(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {

	identities, err := c.Identities(blockID, filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identity (id=%s): %w", nodeID, err)
	}
	if len(identities) != 1 {
		return nil, fmt.Errorf("found invalid number (%d) of identities for node id (%s)", len(identities), nodeID)
	}
	return identities[0], nil
}

func (c *Cluster) LeaderForView(view uint64) (flow.Identifier, error) {

	index, err := c.selection.LeaderIndexForView(view)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get leader for view: %w", err)
	}
	return c.identities[index].NodeID, nil
}

func (c *Cluster) Self() flow.Identifier {
	return c.me
}

func (c *Cluster) DKG(_ flow.Identifier) (hotstuff.DKG, error) {
	panic("queried DKG of cluster committee")
}
