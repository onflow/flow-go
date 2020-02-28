package hotstuff

import (
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/protocol"
)

// ViewState provides method for querying identities related information by view or block ID
type ViewState struct {
	protocolState protocol.State
	// my own identifier
	myID flow.Identifier
	// identityFilter to find only the consensus members for the cluster
	consensusMembersFilter flow.IdentityFilter
	// the cached all consensus members for finding leaders for a certain view
	allNodes flow.IdentityList
}

// NewViewState creates a new ViewState instance
func NewViewState(protocolState protocol.State, myID flow.Identifier, consensusMembersFilter flow.IdentityFilter) (*ViewState, error) {
	// finding all consensus members
	allNodes, err := protocolState.Final().Identities(consensusMembersFilter)
	if err != nil {
		return nil, fmt.Errorf("cannot find all consensus member nodes when initializing ViewState: %w", err)
	}

	if len(allNodes) == 0 {
		return nil, fmt.Errorf("require non-empty consensus member nodes to initialize ViewState")
	}

	return &ViewState{
		protocolState:          protocolState,
		myID:                   myID,
		consensusMembersFilter: consensusMembersFilter,
		allNodes:               allNodes,
	}, nil
}

// IsSelf returns if the given nodeID is myself
func (v *ViewState) IsSelf(nodeID flow.Identifier) bool {
	return nodeID == v.myID
}

// IsSelfLeaderForView returns if myself is the leader at a given view
func (v *ViewState) IsSelfLeaderForView(view uint64) bool {
	return v.IsSelf(v.LeaderForView(view).ID())
}

// ConsensusIdentities returns all the staked nodes for my role at a certain block.
// blockID - specifies the block to be queried.
// nodeIDs - only return identities that matches the given nodeIDs.
func (v *ViewState) ConsensusIdentities(blockID flow.Identifier, nodeIDs ...flow.Identifier) (flow.IdentityList, error) {
	// create filters
	filters := []flow.IdentityFilter{v.consensusMembersFilter, filter.HasStake}
	if len(nodeIDs) > 0 {
		filters = append(filters, filter.HasNodeID(nodeIDs...))
	}

	// query all staked identities
	identities, err := v.protocolState.AtBlockID(blockID).Identities(filters...)
	if err != nil {
		return nil, fmt.Errorf("cannot find consensus identities: %w", err)
	}

	// build the indice map for sorting
	indices := make(map[flow.Identifier]int)
	for i, nodeID := range nodeIDs {
		indices[nodeID] = i
	}

	// sort the identities to be the same order as nodeIDs
	sort.Slice(identities, func(i int, j int) bool {
		return indices[identities[i].NodeID] < indices[identities[j].NodeID]
	})
	return identities, nil
}

// LeaderForView returns the identity of the leader at given view
func (v *ViewState) LeaderForView(view uint64) *flow.Identity {
	leader := roundRobin(v.allNodes, view)
	return leader
}

// Selects Leader in Round-Robin fashion. NO support for Epochs.
func roundRobin(nodes flow.IdentityList, view uint64) *flow.Identity {
	return nodes[int(view)%int(len(nodes))]
}

// ComputeStakeThresholdForBuildingQC returns the stake that is minimally required for building a QC
func ComputeStakeThresholdForBuildingQC(totalStake uint64) uint64 {
	// Given totalStake, we need smallest integer t such that 2 * totalStake / 3 < t
	// Formally, the minimally required stake is: 2 * Floor(totalStake/3) + max(1, totalStake mod 3)
	floorOneThird := totalStake / 3 // integer division, includes floor
	res := 2 * floorOneThird
	divRemainder := totalStake % 3
	if divRemainder <= 1 {
		res = res + 1
	} else {
		res += divRemainder
	}
	return res
}
