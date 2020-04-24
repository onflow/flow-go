// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package members

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// Snapshot
type Snapshot struct {
	protocolStateSnapshot  protocol.Snapshot
	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster
	blockID                flow.Identifier
}

func NewSnapshot(blockID flow.Identifier, protocolStateSnapshot protocol.Snapshot, consensusMembersFilter flow.IdentityFilter) *Snapshot {
	return &Snapshot{
		protocolStateSnapshot:  protocolStateSnapshot,
		consensusMembersFilter: consensusMembersFilter,
		blockID:                blockID,
	}
}

func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	identities, err := s.protocolStateSnapshot.Identities(filter.And(s.consensusMembersFilter, selector))
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", s.blockID, err)
	}
	return identities, nil
}

func (s *Snapshot) Identity(participantID flow.Identifier) (*flow.Identity, error) {
	identity, err := s.protocolStateSnapshot.Identity(participantID)
	if err != nil {
		// ToDo: differentiate between internal error and participantID not being found
		return nil, fmt.Errorf("%x is not a valid node ID at block %x: %w", participantID, s.blockID, model.ErrInvalidConsensusParticipant)
	}
	if !s.consensusMembersFilter(identity) { // participantID is not a consensus participant
		return nil, fmt.Errorf("node %x has wrong role or zero stake at block %x: %w", participantID, s.blockID, model.ErrInvalidConsensusParticipant)
	}
	return identity, nil
}

type Members struct {
	protocolState protocol.State

	//TODO: ultimately, the own identity of the node is necessary for signing.
	//      Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	myID flow.Identifier // my own identifier

	// The constant set of consensus members for the entire Epoch.
	// HotStuff requires that the primary for a view is fork-independent and only depend
	// on the view number. Therefore, all nodes that were part of the initially released list of consensus
	// nodes for the current Epoch retain their spot as primaries for the respective views (even if they
	// are slashed!). Hence, we cache the initial list of consensus nodes for the current Epoch and compute
	// primaries with respect to this list.
	// TODO: very simple implementation; will be updated when introducing Epochs
	epochConsensusMembers flow.IdentityList

	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster
}

func NewState(protocolState protocol.State, myID flow.Identifier) (*Members, error) {
	// finding all consensus members
	epochConsensusMembers, err := protocolState.Final().Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve list of consensus members: %w", err)
	}
	if len(epochConsensusMembers) == 0 {
		return nil, fmt.Errorf("require non-empty consensus member nodes to initialize ViewState")
	}

	return &Members{
		protocolState:          protocolState,
		myID:                   myID,
		epochConsensusMembers:  epochConsensusMembers,
		consensusMembersFilter: filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true)),
	}, nil
}

func (m *Members) AtBlockID(blockID flow.Identifier) hotstuff.MembersSnapshot {
	snapshot := m.protocolState.AtBlockID(blockID)
	return NewSnapshot(blockID, snapshot, m.consensusMembersFilter)
}

// LeaderForView returns the identity of the leader for a given view.
// Currently: deterministic round robin
// CAUTION: naive implementation! NOT compatible with Epochs; can lead to liveness problems
//          when fraction of offline (Byzantine) nodes approaches 1/3
// TODO replace with randomized, stake-proportional, Epoch-aware algorithm
func (m *Members) LeaderForView(view uint64) (flow.Identifier, error) {
	// As long as there are no Epochs, this implementation will never return an error, as
	// leaders can be pre-determined for every view. This will change, when Epochs are added.
	// The API already contains the error return parameter, to be future-proof.
	leaderIndex := int(view) % len(m.epochConsensusMembers)
	leaderIdentity := m.epochConsensusMembers[leaderIndex]
	return leaderIdentity.NodeID, nil
}

// Self returns our own node identifier.
// TODO: ultimately, the own identity of the node is necessary for signing.
//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
//       This would require some refactoring of EventHandler (postponed to later)
func (m *Members) Self() flow.Identifier {
	return m.myID
}
