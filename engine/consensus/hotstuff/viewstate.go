package hotstuff

import (
	"math/big"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protocol"
)

// ViewState provides method for querying identities related information by view or block ID
type ViewState struct {
	protocolState protocol.State
	// my own identifier
	myID flow.Identifier
	// identityFilter to find only the consensus members for the cluster
	consensusMembersFilter flow.IdentityFilter
}

// IsSelf returns if the given nodeID is myself
func (v *ViewState) IsSelf(nodeID flow.Identifier) bool {
	return nodeID == v.myID
}

// IsSelfLeaderForView returns if myself is the leader at a given view
func (v *ViewState) IsSelfLeaderForView(view uint64) bool {
	return v.IsSelf(v.LeaderForView(view).ID())
}

// GetStakedIdentitiesAtBlock returns all the staked nodes for my role at a certain block.
// blockID - specifies the block to be queried.
// nodeIDs - optional arguments to only return identities that matches the given nodeIDs.
func (v *ViewState) GetStakedIdentitiesAtBlock(blockID flow.Identifier, nodeIDs ...flow.Identifier) (flow.IdentityList, error) {
	// filter only the given nodes
	nodeFilter := toNodeFilter(nodeIDs...)
	return v.protocolState.AtBlockID(blockID).Identities(v.consensusMembersFilter, stakedFilter, nodeFilter)
}

// GetQCStakeThresholdAtBlock returns the stack threshold for building QC at a given block
func (v *ViewState) GetQCStakeThresholdAtBlock(blockID flow.Identifier) (uint64, error) {
	// get all the staked nodes
	identities, err := v.GetStakedIdentitiesAtBlock(blockID)
	if err != nil {
		return 0, err
	}
	return ComputeStakeThresholdForBuildingQC(identities.TotalStake()), nil
}

// LeaderForView returns the identity of the leader at given view
func (v *ViewState) LeaderForView(view uint64) *flow.Identity {
	panic("TODO")
}

// ComputeStakeThresholdForBuildingQC returns the threshold to determine how much stake are needed for building a QC
// identities is the full identity list at a certain block
func ComputeStakeThresholdForBuildingQC(totalStake uint64) uint64 {
	// total * 2 / 3
	total := new(big.Int).SetUint64(totalStake)
	two := new(big.Int).SetUint64(2)
	three := new(big.Int).SetUint64(3)
	return new(big.Int).Div(
		new(big.Int).Mul(total, two),
		three).Uint64()
}

// filter only nodes that have stake
func stakedFilter(node *flow.Identity) bool {
	return node.Stake > 0
}

// builds a map from nodeIDs for filter nodes
func toNodeFilter(nodeIDs ...flow.Identifier) flow.IdentityFilter {
	nodeMap := make(map[flow.Identifier]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodeMap[nodeID] = struct{}{}
	}

	return func(node *flow.Identity) bool {
		_, found := nodeMap[node.NodeID]
		return found
	}
}
