package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/protocol"
)

// ViewState provides method for querying identities related information by view or block ID
type ViewState struct {
	protocolState protocol.State

	myID                   flow.Identifier     // my own identifier
	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster
	allNodes               flow.IdentityList   // the cached all consensus members for finding leaders for a certain view

	dkgPublicData *DKGPublicData
}

// DKGPublicData is the public data for DKG participants who generated their key shares
type DKGPublicData struct {
	GroupPubKey           crypto.PublicKey                    // the group public key
	IdToDKGParticipantMap map[flow.Identifier]*DKGParticipant // the mapping from DKG participants Identifier to its full DKGParticipant info
}

// DKGParticipant contains an individual participant's DKG data
type DKGParticipant struct {
	Id             flow.Identifier
	PublicKeyShare crypto.PublicKey
	DKGIndex       int
}

// NewViewState creates a new ViewState instance
func NewViewState(protocolState protocol.State, dkgPublicData *DKGPublicData, myID flow.Identifier, consensusMembersFilter flow.IdentityFilter) (*ViewState, error) {
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
		dkgPublicData:          dkgPublicData,
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

// DKGPublicData returns the public DKG data for block
func (v *ViewState) DKGPublicData() *DKGPublicData {
	return v.dkgPublicData
}

// ConsensusParticipants returns all _staked_ consensus participants at block with blockID.
// Which node is considered an eligible consensus participant is determined by
// `ViewState.consensusMembersFilter` (defined at construction time).
func (v *ViewState) AllConsensusParticipants(blockID flow.Identifier) (flow.IdentityList, error) {
	// create filters
	filters := []flow.IdentityFilter{v.consensusMembersFilter, filter.HasStake}
	// query all staked consensus participants
	identities, err := v.protocolState.AtBlockID(blockID).Identities(filters...)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}
	return identities, nil
}

// IdentityForConsensusParticipant returns the flow.Identity corresponding to the consensus participant
// with ID `participantId`. Errors, if participantId is not a valid and staked consensus participant
// at blockID, this method error. Which node is considered an eligible consensus participant is
// determined by `ViewState.consensusMembersFilter` (defined at construction time).
func (v *ViewState) IdentityForConsensusParticipant(blockID flow.Identifier, participantId flow.Identifier) (*flow.Identity, error) {
	id, err := v.protocolState.AtBlockID(blockID).Identity(participantId)
	if err != nil {
		return nil, fmt.Errorf("error retrieving identity for %s: %w", participantId, err)
	}
	if !v.consensusMembersFilter(id) { // participantId is not a consensus participant
		return nil, fmt.Errorf("not a consensus participant: %s", participantId)
	}
	if id.Stake == 0 {
		return nil, fmt.Errorf("not a staked node: %s", participantId)
	}
	return id, nil
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
func (v *ViewState) IdentitiesForConsensusParticipants(blockID flow.Identifier, consensusNodeIDs ...flow.Identifier) (flow.IdentityList, error) {
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
	filters := []flow.IdentityFilter{v.consensusMembersFilter, filter.HasStake, filter.HasNodeID(consensusNodeIDs...)}
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
