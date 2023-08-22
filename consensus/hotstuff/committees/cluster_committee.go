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
// information from the main chain. Thus, block ID parameters in this DynamicCommittee
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
	// initial set of cluster members, WITHOUT dynamic weight changes
	initialClusterMembers    flow.IdentitySkeletonList
	initialClusterIdentities flow.IdentityList
	weightThresholdForQC     uint64 // computed based on initial cluster committee weights
	weightThresholdForTO     uint64 // computed based on initial cluster committee weights
}

var _ hotstuff.Replicas = (*Cluster)(nil)
var _ hotstuff.DynamicCommittee = (*Cluster)(nil)

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

	totalWeight := cluster.Members().ToSkeleton().TotalWeight()
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
		initialClusterMembers:    cluster.Members().ToSkeleton(),
		initialClusterIdentities: cluster.Members(),
		weightThresholdForQC:     WeightThresholdToBuildQC(totalWeight),
		weightThresholdForTO:     WeightThresholdToTimeout(totalWeight),
	}
	return com, nil
}

// IdentitiesByBlock returns the identities of all cluster members that are authorized to
// participate at the given block. The order of the identities is the canonical order.
func (c *Cluster) IdentitiesByBlock(blockID flow.Identifier) (flow.IdentityList, error) {
	// blockID is a collection block not a block produced by consensus,
	// to query the identities from protocol state, we need to use the reference block id from the payload
	//
	// first retrieve the cluster block payload
	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster payload: %w", err)
	}

	// an empty reference block ID indicates a root block
	isRootBlock := payload.ReferenceBlockID == flow.ZeroID

	// use the initial cluster members for root block
	if isRootBlock {
		return c.initialClusterIdentities, nil
	}

	// otherwise use the snapshot given by the reference block
	identities, err := c.state.AtBlockID(payload.ReferenceBlockID).Identities(c.clusterMemberFilter)
	return identities, err
}

func (c *Cluster) IdentityByBlock(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {

	// first retrieve the cluster block payload
	payload, err := c.payloads.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get cluster payload: %w", err)
	}

	// an empty reference block ID indicates a root block
	isRootBlock := payload.ReferenceBlockID == flow.ZeroID

	// use the initial cluster members for root block
	if isRootBlock {
		identity, ok := c.initialClusterIdentities.ByNodeID(nodeID)
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

// IdentitiesByEpoch returns the initial cluster members for this epoch. The view
// parameter is the view in the cluster consensus. Since clusters only exist for
// one epoch, we don't need to check the view.
func (c *Cluster) IdentitiesByEpoch(_ uint64) (flow.IdentitySkeletonList, error) {
	return c.initialClusterMembers, nil
}

// IdentityByEpoch returns the node from the initial cluster members for this epoch.
// The view parameter is the view in the cluster consensus. Since clusters only exist
// for one epoch, we don't need to check the view.
//
// Returns:
//   - model.InvalidSignerError if nodeID was not listed by the Epoch Setup event as an
//     authorized participant in this cluster
func (c *Cluster) IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.IdentitySkeleton, error) {
	identity, ok := c.initialClusterMembers.ByNodeID(participantID)
	if !ok {
		return nil, model.NewInvalidSignerErrorf("node %v is not an authorized hotstuff participant", participantID)
	}
	return identity, nil
}

func (c *Cluster) LeaderForView(view uint64) (flow.Identifier, error) {
	return c.selection.LeaderForView(view)
}

// QuorumThresholdForView returns the weight threshold required to build a QC
// for the given view. The view parameter is the view in the cluster consensus.
// Since clusters only exist for one epoch, and the weight threshold is static
// over the course of an epoch, we don't need to check the view.
//
// No errors are expected during normal operation.
func (c *Cluster) QuorumThresholdForView(_ uint64) (uint64, error) {
	return c.weightThresholdForQC, nil
}

// TimeoutThresholdForView returns the minimum weight of observed timeout objects to
// safely immediately timeout for the current view. The view parameter is the view
// in the cluster consensus. Since clusters only exist for one epoch, and the weight
// threshold is static over the course of an epoch, we don't need to check the view.
//
// No errors are expected during normal operation.
func (c *Cluster) TimeoutThresholdForView(_ uint64) (uint64, error) {
	return c.weightThresholdForTO, nil
}

func (c *Cluster) Self() flow.Identifier {
	return c.me
}

func (c *Cluster) DKG(_ uint64) (hotstuff.DKG, error) {
	panic("queried DKG of cluster committee")
}
