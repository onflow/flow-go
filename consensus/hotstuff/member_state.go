// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

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
	//    * ErrInvalidConsensusParticipant if participantID does not correspond to a _staked_ consensus member at the specified block.
	Identity(participantID flow.Identifier) (*flow.Identity, error)
}

// ConsensusMembers manages the consensus' MembersSnapshot for the different blocks.
// It allows to retrieve MembersSnapshots of the state at any point of the protocol.State history.
type MembersState interface {
	// AtBlockID returns a the protocol state of all legitimate consensus participants for the specified block.
	// It is available for any block that was introduced into the protocol state. Hence, the snapshot
	// can thus represent an ambiguous state that was or will never be finalized.
	AtBlockID(blockID flow.Identifier) MembersSnapshot

	// LeaderForView returns the identity of the leader for a given view.
	// Can error if view is in a future Epoch for which the consensus committee hasn't been determined yet.
	LeaderForView(view uint64) (flow.Identifier, error)

	// IsSelf returns true if and only the nodeID refers to this node.
	// TODO: ultimately, the own identity of the node is necessary for signing.
	//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	//       This would require some refactoring of EventHandler (postponed to later)
	IsSelf(nodeID flow.Identifier) bool

	// Self returns our own node identifier.
	Self() flow.Identifier
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
