// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package viewstate

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// DefaultParticipantState
//
type DefaultParticipantState struct {
	protocolState          protocol.State
	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster

	// the constant set of consensus members for the entire Epoch
	// This is necessary because HotStuff requires that the primary for a view is fork-independent and only depend
	// on the view number. Therefore, all nodes that were part of the initially released list of consensus
	// nodes for the current Epoch retain their spot as primaries for the respective view numbers (even if they
	// are slashed!). Hence, we cache the initial list of consensus nodes for the current Epoch and compute
	// primaries with respect to this list.
	epochConsensusMembers flow.IdentityList
}

func NewParticipantState(protocolState protocol.State) ParticipantState {
	consensusMembersFilter := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))
	return &DefaultParticipantState{
		protocolState:          protocolState,
		consensusMembersFilter: consensusMembersFilter,
	}
}

func (d *DefaultParticipantState) ParticipantsIdentities(blockID flow.Identifier) (flow.IdentityList, error) {
	identities, err := d.protocolState.AtBlockID(blockID).Identities(d.consensusMembersFilter)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}
	return identities, nil
}

func (d *DefaultParticipantState) ParticipantIdentity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	identity, err := d.protocolState.AtBlockID(blockID).Identity(participantID)
	if err != nil {
		return nil, model.ErrorInvalidConsensusParticipants{blockID, []flow.Identifier{participantID}, err.Error()}
	}
	if !d.consensusMembersFilter(identity) { // participantID is not a consensus participant
		return nil, model.ErrorInvalidConsensusParticipants{blockID, []flow.Identifier{participantID}, "node has wrong role or zero stake"}
	}
	return identity, nil
}

func (d *DefaultParticipantState) LeaderForView(view uint64) *flow.Identity {
	// Selects Leader in Round-Robin fashion. NO support for Epochs.
	// TODO replace with randomized, stake-proportional, Epoch-aware algorithm
	leaderIndex := int(view) % len(d.epochConsensusMembers)
	return d.epochConsensusMembers[leaderIndex]
}
