package viewstate

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// ViewState provides method for querying identities related information by view or block ID
type ViewState struct {
	protocolState protocol.State

	myID                   flow.Identifier     // my own identifier
	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster
	allNodes               flow.IdentityList   // the cached all consensus members for finding leaders for a certain view

	dkgState dkg.State
}

// New creates a new ViewState instance
func New(protocolState protocol.State, dkgState dkg.State, myID flow.Identifier, consensusMembersFilter flow.IdentityFilter) (*ViewState, error) {
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
		dkgState:               dkgState,
	}, nil
}

// Self returns the own nodeID.
func (v *ViewState) Self() flow.Identifier {
	return v.myID
}

// IsSelf returns if the given nodeID is myself
func (v *ViewState) IsSelf(nodeID flow.Identifier) bool {
	return nodeID == v.myID
}

// IsSelfLeaderForView returns if myself is the leader at a given view
func (v *ViewState) IsSelfLeaderForView(view uint64) bool {
	return v.IsSelf(v.LeaderForView(view).ID())
}

// DKGState returns the public DKG data for block
func (v *ViewState) DKGState() dkg.State {
	return v.dkgState
}

// ConsensusParticipants returns all _staked_ consensus participants at block with blockID.
// Which node is considered an eligible consensus participant is determined by
// `ViewState.consensusMembersFilter` (defined at construction time).
func (v *ViewState) AllConsensusParticipants(blockID flow.Identifier) (flow.IdentityList, error) {
	// create filters
	filters := []flow.IdentityFilter{v.consensusMembersFilter, filter.HasStake(true)}
	// query all staked consensus participants
	identities, err := v.protocolState.AtBlockID(blockID).Identities(filters...)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}
	return identities, nil
}

// IdentityForConsensusParticipant returns the flow.Identity corresponding to the consensus participant
// with ID `participantID`. Errors, if participantID is not a valid and staked consensus participant
// at blockID, this method error. Which node is considered an eligible consensus participant is
// determined by `ViewState.consensusMembersFilter` (defined at construction time).
// ERROR conditions:
//   * INVALID CONSENSUS MEMBER: participantID does NOT correspond to staked consensus participant
func (v *ViewState) IdentityForConsensusParticipant(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	identity, err := v.protocolState.AtBlockID(blockID).Identity(participantID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving identity for %s: %w", participantID, err)
	}
	if !v.consensusMembersFilter(identity) { // participantID is not a consensus participant
		return nil, fmt.Errorf("not a consensus participant: %s", participantID)
	}
	if identity.Stake == 0 {
		return nil, fmt.Errorf("not a staked node: %s", participantID)
	}
	return identity, nil
}

// IdentitiesForConsensusParticipant translates the given consensus IDs to flow.Identifiers.
//    blockID - specifies the block to be queried.
//    consensusNodeIDs - nodeIDs of consensus nodes
// Return:
//    List L := flow.IdentityList where L[k] is the flow.Identity for consensusNodeIDs[k]
//    error: if any consensusNodeIDs[k] does not correspond to a _staked_ consensus member at blockID
// Intended application:
//    * counting stake. Hence, we don't want duplicated identities (i.e. we just error)
//
// Caution:
//   * PRESERVED ORDER: each element in `consensusNodeIDs` is expected to be a valid consensus member,
//     i.e. pass the `ViewState.consensusMembersFilter` (defined at construction time)
// ERROR conditions:
//   * DUPLICATES: consensusNodeIDs contains duplicates
//   * DUPLICATES: an element in consensusNodeIDs does NOT correspond to staked consensus node
// Properties
//   * PRESERVES ORDER of `consensusNodeIDs`, i.e. for the returned list L, we have L[k] is the flow.Identity for consensusNodeIDs[k]
// ERROR conditions:
//   * DUPLICATES: consensusNodeIDs contains duplicates
//   * INVALID CONSENSUS MEMBER: an element in consensusNodeIDs does NOT correspond to staked consensus participant
func (v *ViewState) IdentitiesForConsensusParticipants(blockID flow.Identifier, consensusNodeIDs []flow.Identifier) (flow.IdentityList, error) {
	if len(consensusNodeIDs) == 0 { // Special case: consensusNodeIDs is empty
		// _no_ filter will be applied and all consensus participants are returned.
		return v.AllConsensusParticipants(blockID)
	}
	if len(consensusNodeIDs) == 1 { // Special case: consensusNodeIDs is single
		// Theoretically, this case is correctly computed, by the logic below. However, the logic below will
		// go over _all_ consensus nodes and just retain the single input.
		// In contrast, IdentityForConsensusParticipant allows for a potentially more efficient DataBase lookup.
		// Hence, we call into the potentially optimized logic here:
		identity, err := v.IdentityForConsensusParticipant(blockID, consensusNodeIDs[0])
		if err != nil {
			return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
		}
		return []*flow.Identity{identity}, nil
	}

	// Retrieve full flow.Identity for each element in consensusNodeIDs:
	// Select Identities at block via Filters: consensus participants, staked, element of consensusNodeIDs
	filters := []flow.IdentityFilter{v.consensusMembersFilter, filter.HasStake(true), filter.HasNodeID(consensusNodeIDs...)}
	consensusIdentities, err := v.protocolState.AtBlockID(blockID).Identities(filters...)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}

	// create a lookup for staked consensus identities by ID
	lookup := make(map[flow.Identifier]*flow.Identity)
	for _, identity := range consensusIdentities {
		lookup[identity.ID()] = identity
	}
	if len(lookup) != len(consensusNodeIDs) { // fail fast if there are missing or duplicated identities
		return nil, fmt.Errorf("consensusNodeIDs might contain duplicated or unstaked identities")
	}

	// create a list of identities to populate based on ordering of consensusNodeIDs
	orderedConsensusIdentities := make([]*flow.Identity, 0, len(consensusIdentities))
	for _, nodeID := range consensusNodeIDs {
		// By construction, we _always_ have the Identity for nodeID in the lookup:
		// lookup is a subset of consensusNodeIDs.
		orderedConsensusIdentities = append(orderedConsensusIdentities, lookup[nodeID])
	}
	return orderedConsensusIdentities, nil
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
