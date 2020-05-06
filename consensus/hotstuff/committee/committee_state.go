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

// BlockTranslator is a support function for determining the protocol for the current hotstuff instance.
// Only for the main consensus, their blocks can be directly used to retrieve protocol.state Snapshots.
// Hence, For main consensus, this is an identity method f(x) = x.
// For collector consensus, the collector Blocks need to be translated to some reference block on the main chain.
// Ideally, it's the most recently finalized block on the main chain.
type BlockTranslator func(blockID *flow.Identifier) (*flow.Identifier, error)

// Committee accounts for the fact that we might have multiple HotStuff instances
// (collector committees and main consensus committee). Each hostuff instance is supposed to
// have a dedicated Committee state.
// A Committee provides subset of the protocol.State, which is restricted to exactly those
// nodes that participate in the current HotStuff instance: the state of all legitimate HotStuff
// participants for the specified block. Legitimate HotStuff participants have NON-ZERO STAKE.
//
// The intended use case is to support collectors running HotStuff within Flow. Specifically,
// the collectors produced their own blocks, independently of the Consensus Nodes (aka the main consensus).
// Given a collector block, some logic is required to find the main consensus block
// for determining the valid collector-HotStuff participants.
type Committee struct {
	protocolState   protocol.State
	blockTranslator BlockTranslator

	membersFilter flow.IdentityFilter // identityFilter to find only the members for this particular HotStuff instance

	// The constant set of HotStuff members for the entire Epoch.
	// HotStuff requires that the primary for a view is fork-independent and only depend
	// on the view number. Therefore, all nodes that were part of the initially released list of HotStuff
	// members for the current Epoch retain their spot as primaries for the respective views (even if they
	// are slashed!). Hence, we cache the initial list of HotStuff nodes for the current Epoch and compute
	// primaries with respect to this list.
	// TODO: very simple implementation; will be updated when introducing Epochs
	epochParticipants flow.IdentifierList

	//TODO: ultimately, the own identity of the node is necessary for signing.
	//      Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	myID flow.Identifier // my own identifier
}

// Identities returns a IdentityList with legitimate HotStuff participants for the specified block.
// The list of participants is filtered by the provided selector. The returned list of HotStuff participants
//   * contains nodes that are allowed to sign the specified block (legitimate participants with NON-ZERO STAKE)
//   * is ordered in the canonical order
//   * contains no duplicates.
// The list of all legitimate HotStuff participants for the specified block can be obtained by using `filter.Any`
func (c *Committee) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	mainConsensusBlockID, err := c.blockTranslator(&blockID)
	if err != nil {
		return nil, fmt.Errorf("error determining the protocol state for block %s: %w", blockID, err)
	}
	identities, err := c.protocolState.AtBlockID(*mainConsensusBlockID).Identities(
		filter.And(c.membersFilter, selector),
	)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants for block %s: %w", blockID, err)
	}
	return identities, nil
}

// Identity returns the full Identity for specified HotStuff participant.
// The node must be a legitimate HotStuff participant with NON-ZERO STAKE at the specified block.
// ERROR conditions:
//    * ErrInvalidSigner if participantID does NOT correspond to a _staked_ HotStuff participant at the specified block.
func (c *Committee) Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	mainConsensusBlockID, err := c.blockTranslator(&blockID)
	if err != nil {
		return nil, fmt.Errorf("error determining the protocol state for block %s: %w", blockID, err)
	}
	identity, err := c.protocolState.AtBlockID(*mainConsensusBlockID).Identity(participantID)
	if err != nil {
		// ToDo: differentiate between internal error and participantID not being found
		return nil, fmt.Errorf("%x is not a valid node ID at block %x: %w", participantID, blockID, model.ErrInvalidSigner)
	}
	if !c.membersFilter(identity) { // participantID is not a consensus participant
		return nil, fmt.Errorf("node %x has wrong role or zero stake at block %x: %w", participantID, blockID, model.ErrInvalidSigner)
	}
	return identity, nil
}

// LeaderForView returns the identity of the leader for a given view.
// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
//          Therefore, a node retains its proposer view slots even if it is slashed.
//          Its proposal is simply considered invalid, as it is not from a legitimate participant.
// Can error if view is in a future Epoch for which the HotStuff committee hasn't been determined yet.
func (c *Committee) LeaderForView(view uint64) (flow.Identifier, error) {
	// As long as there are no Epochs, this implementation will never return an error, as
	// leaders can be pre-determined for every view. This will change, when Epochs are added.
	// The API already contains the error return parameter, to be future-proof.
	leaderIndex := int(view) % len(c.epochParticipants)
	return c.epochParticipants[leaderIndex], nil
}

// Self returns our own node identifier.
// TODO: ultimately, the own identity of the node is necessary for signing.
//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
//       This would require some refactoring of EventHandler (postponed to later)
func (c *Committee) Self() flow.Identifier {
	return c.myID
}

// New creates HotStuff committee. This is the generic constructor covering all potential cases.
// It requires:
//    * protocolState: the protocol state for the entire network
//    * blockTranslator: translates a block from the current HotStuff committee to a block of the main consensus committee to determine legitimacy of nodes
//    * myID: ID of the current node. CAUTION: this does not make the current node part of the HotStuff committee
//    * filter: filter which retains only legitimate participants for the current consensus instance.
//      It must filter for: node role, non-zero stake, and potentially the collector cluster (if applicable)
//    * epochParticipants: all nodes that were part of the initially released list of participants for this HotStuff instance.
//	    All participants for the current Epoch retain their spot as primaries for the respective views (even if they are slashed!)
//
// While you can use this constructor to generate a committee state for the main consensus,
// the function `NewMainConsensusCommitteeState` provides a more concise API.
func New(protocolState protocol.State, blockTranslator BlockTranslator, myID flow.Identifier, filter flow.IdentityFilter, epochParticipants flow.IdentifierList) hotstuff.Committee {
	return &Committee{
		protocolState:     protocolState,
		blockTranslator:   blockTranslator,
		membersFilter:     filter,
		epochParticipants: epochParticipants,
		myID:              myID,
	}
}

// NewMainConsensusCommitteeState creates HotStuff committee consisting of the MAIN CONSENSUS NODES.
// It requires:
//    * protocolState: the protocol state for the entire network
//    * myID: ID of the current node. CAUTION: this does not make the current node part of the HotStuff committee
//
// For constructing committees for other HotStuff instances (such as collector HotStuff instances), please use the
// generic `New` function.
func NewMainConsensusCommitteeState(protocolState protocol.State, myID flow.Identifier) (hotstuff.Committee, error) {
	// finding all consensus members
	epochConsensusMembers, err := protocolState.Final().Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve the consensus committee: %w", err)
	}
	if len(epochConsensusMembers) == 0 {
		return nil, fmt.Errorf("require non-empty consensus member nodes to initialize ViewState")
	}

	blockTranslator := func(blockID *flow.Identifier) (*flow.Identifier, error) { return blockID, nil }
	consensusNodeFilter := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))
	return New(protocolState, blockTranslator, myID, consensusNodeFilter, epochConsensusMembers.NodeIDs()), nil
}
