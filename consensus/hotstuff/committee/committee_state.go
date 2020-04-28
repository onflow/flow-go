// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package committee

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/state/protocol"
)

type BlockTranslator func(blockID flow.Identifier) flow.Identifier

// Committee implements hotstuff.Committee
type Committee struct {
	protocolState   protocol.State
	blockTranslator BlockTranslator

	consensusMembersFilter flow.IdentityFilter // identityFilter to find only the consensus members for the cluster

	// The constant set of consensus members for the entire Epoch.
	// HotStuff requires that the primary for a view is fork-independent and only depend
	// on the view number. Therefore, all nodes that were part of the initially released list of consensus
	// nodes for the current Epoch retain their spot as primaries for the respective views (even if they
	// are slashed!). Hence, we cache the initial list of consensus nodes for the current Epoch and compute
	// primaries with respect to this list.
	// TODO: very simple implementation; will be updated when introducing Epochs
	epochConsensusMembers flow.IdentifierList

	//TODO: ultimately, the own identity of the node is necessary for signing.
	//      Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	myID flow.Identifier // my own identifier
}

// Identities returns a IdentityList with legitimate consensus participants for the specified block.
// The list of participants is filtered by the provided selector. The returned list of consensus participants
//   * contains nodes that are allowed to sign the specified block (legitimate consensus participants with NON-ZERO STAKE)
//   * is ordered in the canonical order
//   * contains no duplicates.
// The list of all legitimate consensus participants for the specified block can be obtained by using `filter.Any`
func (c *Committee) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	mainConsensusBlockID := c.blockTranslator(blockID)
	identities, err := c.protocolState.AtBlockID(mainConsensusBlockID).Identities(
		filter.And(c.consensusMembersFilter, selector),
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}
	return identities, nil
}

// Identity returns the full Identity for specified consensus participant.
// The node must be a legitimate consensus participant with NON-ZERO STAKE at the specified block.
// ERROR conditions:
//    * ErrInvalidSigner if participantID does not correspond to a _staked_ consensus member at the specified block.
func (c *Committee) Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	mainConsensusBlockID := c.blockTranslator(blockID)
	identity, err := c.protocolState.AtBlockID(mainConsensusBlockID).Identity(participantID)
	if err != nil {
		// ToDo: differentiate between internal error and participantID not being found
		return nil, fmt.Errorf("%x is not a valid node ID at block %x: %w", participantID, blockID, model.ErrInvalidSigner)
	}
	if !c.consensusMembersFilter(identity) { // participantID is not a consensus participant
		return nil, fmt.Errorf("node %x has wrong role or zero stake at block %x: %w", participantID, blockID, model.ErrInvalidSigner)
	}
	return identity, nil
}

// LeaderForView returns the identity of the leader for a given view.
// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
//          Therefore, a node retains its proposer view slots even if it is slashed.
//          Its proposal is simply considered invalid, as it is not from a legitimate consensus participant.
// Can error if view is in a future Epoch for which the consensus committee hasn't been determined yet.
func (c *Committee) LeaderForView(view uint64) (flow.Identifier, error) {
	// As long as there are no Epochs, this implementation will never return an error, as
	// leaders can be pre-determined for every view. This will change, when Epochs are added.
	// The API already contains the error return parameter, to be future-proof.
	leaderIndex := int(view) % len(c.epochConsensusMembers)
	return c.epochConsensusMembers[leaderIndex], nil
}

// Self returns our own node identifier.
// TODO: ultimately, the own identity of the node is necessary for signing.
//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
//       This would require some refactoring of EventHandler (postponed to later)
func (c *Committee) Self() flow.Identifier {
	return c.myID
}

func New(protocolState protocol.State, blockTranslator BlockTranslator, myID flow.Identifier, filter flow.IdentityFilter, epochConsensusMembers flow.IdentifierList) hotstuff.Committee {
	return &Committee{
		protocolState:          protocolState,
		blockTranslator:        blockTranslator,
		consensusMembersFilter: filter,
		epochConsensusMembers:  epochConsensusMembers,
		myID:                   myID,
	}
}

func NewMainConsensusCommitteeState(protocolState protocol.State, myID flow.Identifier) (hotstuff.Committee, error) {
	// finding all consensus members
	epochConsensusMembers, err := protocolState.Final().Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve the consensus committee: %w", err)
	}
	if len(epochConsensusMembers) == 0 {
		return nil, fmt.Errorf("require non-empty consensus member nodes to initialize ViewState")
	}

	blockTranslator := func(blockID flow.Identifier) flow.Identifier { return blockID }
	consensusNodeFilter := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))
	return New(protocolState, blockTranslator, myID, consensusNodeFilter, epochConsensusMembers.NodeIDs()), nil
}
