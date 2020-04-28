// (c) 2020 Dapper Labs - ALL RIGHTS RESERVED
package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Committee provides a subset of the protocol.State:
// the state of all legitimate consensus participants for the specified block.
// Legitimate consensus participants have NON-ZERO STAKE.
//
// The intended use case is to support collector consensus within Flow. Specifically,
// the collectors produced their own blocks, independently of the Consensus Nodes (aka the main consensus).
// Given a collector block, some logic is required to find the main consensus block
// for determining the valid collector consensus participants.
// This logic is encapsulated in ParticipantState.
type Committee interface {

	// Identities returns a IdentityList with legitimate consensus participants for the specified block.
	// The list of participants is filtered by the provided selector. The returned list of consensus participants
	//   * contains nodes that are allowed to sign the specified block (legitimate consensus participants with NON-ZERO STAKE)
	//   * is ordered in the canonical order
	//   * contains no duplicates.
	// The list of all legitimate consensus participants for the specified block can be obtained by using `filter.Any`
	Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error)

	// Identity returns the full Identity for specified consensus participant.
	// The node must be a legitimate consensus participant with NON-ZERO STAKE at the specified block.
	// ERROR conditions:
	//    * ErrInvalidSigner if participantID does not correspond to a _staked_ consensus member at the specified block.
	Identity(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error)

	// LeaderForView returns the identity of the leader for a given view.
	// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
	//          Therefore, a node retains its proposer view slots even if it is slashed.
	//          Its proposal is simply considered invalid, as it is not from a legitimate consensus participant.
	// Can error if view is in a future Epoch for which the consensus committee hasn't been determined yet.
	LeaderForView(view uint64) (flow.Identifier, error)

	// Self returns our own node identifier.
	// TODO: ultimately, the own identity of the node is necessary for signing.
	//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	//       This would require some refactoring of EventHandler (postponed to later)
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
