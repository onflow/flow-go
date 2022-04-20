package hotstuff

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// A committee provides a subset of the protocol.State, which is restricted to exactly those
// nodes that participate in the current HotStuff instance: the state of all legitimate HotStuff
// participants for the specified view. Legitimate HotStuff participants have NON-ZERO WEIGHT.
//
// For the purposes of validating votes, timeouts, quorum certificates, and timeout certificates
// we consider a committee which is static over the course of an epoch. Although committee
// members may be ejected, or have their weight change during an epoch, we ignore these changes,
// For these purposes we use the VoterCommittee and *ByEpoch methods.
//
// When validating proposals, we take into account changes to the committee during the course of
// an epoch. In particular, if a node is ejected, we will immediately reject all future proposals
// from that node. For these purposes we use the Committee and *ByBlock methods.

// VoterCommittee defines the consensus committee for the purposes of validating votes, timeouts,
// quorum certificates, and timeout certificates. Any consensus committee member who was legitimate
// AT THE BEGINNING of the epoch may produce valid votes and timeouts for the entire epoch, even
// if they are later ejected.
//
// Since the voter committee is considered static over an epoch:
// * we can query identities by view
// * we don't need the full block ancestry prior to validating messages
//
type VoterCommittee interface {
	// LeaderForView returns the identity of the leader for a given view.
	// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
	//          Therefore, a node retains its proposer view slots even if it is slashed.
	//          Its proposal is simply considered invalid, as it is not from a legitimate participant.
	// Returns the following expected errors for invalid inputs:
	//  * epoch containing the requested view has not been set up (protocol.ErrNextEpochNotSetup)
	//  * epoch is too far in the past (leader.InvalidViewError)
	LeaderForView(view uint64) (flow.Identifier, error)

	// Self returns our own node identifier.
	// TODO: ultimately, the own identity of the node is necessary for signing.
	//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	//       This would require some refactoring of EventHandler (postponed to later)
	Self() flow.Identifier

	// DKG returns the DKG info for epoch given by the input view.
	DKG(view uint64) (DKG, error)

	// IdentitiesByEpoch returns a list of the legitimate HotStuff participants for the epoch
	// given by the input view. The list of participants is filtered by the provided selector.
	// The returned list of HotStuff participants:
	//   * contains nodes that are allowed to submit votes or timeouts within the given epoch
	//     (un-ejected, non-zero weight at the beginning of the epoch)
	//   * is ordered in the canonical order
	//   * contains no duplicates.
	// The list of all legitimate HotStuff participants for the given epoch can be obtained by using `filter.Any`
	//
	// CAUTION: DO NOT use this method for validating block proposals.
	// TODO: error for unknown view
	IdentitiesByEpoch(view uint64, selector flow.IdentityFilter) (flow.IdentityList, error)

	// IdentityByEpoch returns the full Identity for specified HotStuff participant.
	// The node must be a legitimate HotStuff participant with NON-ZERO WEIGHT at the specified block.
	// ERROR conditions:
	//  * model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
	//
	// CAUTION: DO NOT use this method for validating block proposals.
	// TODO: error for unknown view
	IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.Identity, error)
}

// Committee extends VoterCommittee to provide the consensus committee for the purposes
// of validating proposals. The proposer committee reflects block-to-block changes in the
// identity table to support immediately rejecting proposals from nodes after they are ejected.
//
// Since the proposer committee can change at any block:
// * we query by block ID
// * we must have incorporated the full block ancestry prior to validating messages
type Committee interface {
	VoterCommittee

	// IdentitiesByBlock returns a list of the legitimate HotStuff participants for the given block.
	// The list of participants is filtered by the provided selector.
	// The returned list of HotStuff participants:
	//   * contains nodes that are allowed to submit proposals, votes, and timeouts
	//     (un-ejected, non-zero weight at current block)
	//   * is ordered in the canonical order
	//   * contains no duplicates.
	// The list of all legitimate HotStuff participants for the given epoch can be obtained by using `filter.Any`
	//
	// TODO - do we need this, if we are only checking a single proposer ID?
	IdentitiesByBlock(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error)

	// IdentityByBlock returns the full Identity for specified HotStuff participant.
	// The node must be a legitimate HotStuff participant with NON-ZERO WEIGHT at the specified block.
	// ERROR conditions:
	//  * model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
	IdentityByBlock(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error)
}

// TODO remove
type DKG interface {
	protocol.DKG
}

// ComputeWeightThresholdForBuildingQC returns the weight that is minimally required for building a QC
func ComputeWeightThresholdForBuildingQC(totalWeight uint64) uint64 {
	// Given totalWeight, we need the smallest integer t such that 2 * totalWeight / 3 < t
	// Formally, the minimally required weight is: 2 * Floor(totalWeight/3) + max(1, totalWeight mod 3)
	floorOneThird := totalWeight / 3 // integer division, includes floor
	res := 2 * floorOneThird
	divRemainder := totalWeight % 3
	if divRemainder <= 1 {
		res = res + 1
	} else {
		res += divRemainder
	}
	return res
}
