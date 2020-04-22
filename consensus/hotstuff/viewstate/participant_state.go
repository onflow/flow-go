// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package viewstate

import "github.com/dapperlabs/flow-go/model/flow"

// MembersSnapshot provides a subset of the protocol.State:
// the state of all legitimate consensus participants for the specified block.
// Legitimate consensus participants have NON-ZERO STAKE.
//
// The intended use case is to support collector consensus within Flow. Specifically,
// the collectors produced their own blocks, independently of the Consensus Nodes (aka the main consensus).
// Given a collector block, some logic is required to find the main consensus block
// for determining the valid collector consensus participants.
// This logic is encapsulated in ParticipantState.
type MembersSnapshot interface {

	// Identities returns a list of staked identities at the selected point of
	// the protocol state history. It allows us to provide optional upfront
	// filters which can be used by the implementation to speed up database
	// lookups.
	Identities(selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity attempts to retrieve the node with the given identifier at the
	// selected point of the protocol state history. It will error if it doesn't
	// exist or if its stake is zero.
	// ERROR conditions:
	//    * ErrorInvalidConsensusParticipants if participantID does not correspond to a _staked_ consensus member at the specified block.
	Identity(participantID flow.Identifier) (*flow.Identity, error)

	// LeaderForView returns the identity of the leader for a given view
	LeaderForView(view uint64) *flow.Identity
}

// ConsensusMembers manages the consensus' MembersSnapshot for the different blocks.
// It allows to retrieve MembersSnapshots of the state at any point of the protocol.State history.
type ConsensusMembers interface {
	// AtBlockID returns a the protocol state of all legitimate consensus participants for the specified block.
	// It is available for any block that was introduced into the protocol state. Hence, the snapshot
	// can thus represent an ambiguous state that was or will never be finalized.
	AtBlockID(blockID flow.Identifier) MembersSnapshot
}
