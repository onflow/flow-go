package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/dkg"
)

type ViewState interface {

	// Self returns our own node identifier.
	Self() flow.Identifier

	// IsSelf checks whether the given node identifier represens ourselves.
	IsSelf(nodeID flow.Identifier) bool

	// IsSelfLeaderForView checks whether we are the leader for the given view.
	IsSelfLeaderForView(view uint64) bool

	// DKGState returns a wrapper API around DKG state information.
	// NOTE: as we currently run with a single epoch, this never changes.
	DKGState() dkg.State

	// AllConsensusParticipants returns a list of the identities of all
	// participants to the consensus algorithm.
	// NOTE: as we don't remove fully slashed nodes and are running on a single
	// epoch at the moment, this list is stable throughout execution.
	AllConsensusParticipants(blockID flow.Identifier) (flow.IdentityList, error)

	// IdentityForConsensusParticipant returns the identity for the node of the
	// given node identifier at the given block, including the up-to-date stoked
	// amount.
	IdentityForConsensusParticipant(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error)

	// IdentitiesForConsensusParticipants returns the identities for the nodes
	// of the given node identifiers at the given block, including the up-to-date
	// staked amounts.
	IdentitiesForConsensusParticipants(blockID flow.Identifier, consensusNodeIDs []flow.Identifier) (flow.IdentityList, error)

	// LeaderForView returns the identity of the leader for the given view.
	LeaderForView(view uint64) *flow.Identity
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
