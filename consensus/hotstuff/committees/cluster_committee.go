package committees

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
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
	state    protocol.State
	payloads storage.ClusterPayloads
	me       flow.Identifier
	// pre-computed leader selection for the full lifecycle of the cluster
	selection *leader.LeaderSelection
	// a filter that returns all members of the cluster committee allowed to vote
	clusterMemberFilter flow.IdentityFilter
	// initial set of cluster members, WITHOUT updated weight
	initialClusterMembers flow.IdentityList
}

var _ hotstuff.Committee = (*Cluster)(nil)

func NewClusterCommittee(
	state protocol.State,
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
		state:     state,
		payloads:  payloads,
		me:        me,
		selection: selection,
		clusterMemberFilter: filter.And(
			cluster.Members().Selector(),
			filter.Not(filter.Ejected),
			filter.HasWeight(true),
		),
		initialClusterMembers: cluster.Members(),
	}
	return com, nil
}

func (c *Cluster) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {

	// first retrieve the cluster block payload
	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster payload: %w", err)
	}

	// an empty reference block ID indicates a root block
	isRootBlock := payload.ReferenceBlockID == flow.ZeroID

	// use the initial cluster members for root block
	if isRootBlock {
		return c.initialClusterMembers.Filter(selector), nil
	}

	// otherwise use the snapshot given by the reference block
	identities, err := c.state.AtBlockID(payload.ReferenceBlockID).Identities(filter.And(
		selector,
		c.clusterMemberFilter,
	))
	return identities, err
}

func (c *Cluster) Identity(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {

	// first retrieve the cluster block payload
	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster payload: %w", err)
	}

	// an empty reference block ID indicates a root block
	isRootBlock := payload.ReferenceBlockID == flow.ZeroID

	// use the initial cluster members for root block
	if isRootBlock {
		identity, ok := c.initialClusterMembers.ByNodeID(nodeID)
		if !ok {
			return nil, model.NewInvalidSignerErrorf("node %v is not an authorized hotstuff participant", nodeID)
		}
		return identity, nil
	}

	// otherwise use the snapshot given by the reference block
	identity, err := c.state.AtBlockID(payload.ReferenceBlockID).Identity(nodeID)
	if protocol.IsIdentityNotFound(err) {
		return nil, model.NewInvalidSignerErrorf("%v is not a valid node id at block %v: %w", nodeID, payload.ReferenceBlockID, err)
	}
	if err != nil {
		return nil, fmt.Errorf("could not get identity for node (id=%x): %w", nodeID, err)
	}
	if !c.clusterMemberFilter(identity) {
		return nil, model.NewInvalidSignerErrorf("node %v is not an authorized hotstuff cluster member", nodeID)
	}
	return identity, nil
}

func (c *Cluster) LeaderForView(view uint64) (flow.Identifier, error) {
	return c.selection.LeaderForView(view)
}

func (c *Cluster) Self() flow.Identifier {
	return c.me
}

func (c *Cluster) DKG(_ flow.Identifier) (hotstuff.DKG, error) {
	panic("queried DKG of cluster committee")
}
