package filter

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// Adapt takes an IdentityFilter on the domain of IdentitySkeletons
// and adapts the filter to the domain of full Identities. In other words, it converts
// flow.IdentityFilter[flow.IdentitySkeleton] to flow.IdentityFilter[flow.Identity].
func Adapt(f flow.IdentityFilter[flow.IdentitySkeleton]) flow.IdentityFilter[flow.Identity] {
	return func(i *flow.Identity) bool {
		return f(&i.IdentitySkeleton)
	}
}

// Any will always be true.
func Any(*flow.Identity) bool {
	return true
}

// And combines two or more filters that all need to be true.
func And[T flow.GenericIdentity](filters ...flow.IdentityFilter[T]) flow.IdentityFilter[T] {
	return func(identity *T) bool {
		for _, filter := range filters {
			if !filter(identity) {
				return false
			}
		}
		return true
	}
}

// Or combines two or more filters and only needs one of them to be true.
func Or[T flow.GenericIdentity](filters ...flow.IdentityFilter[T]) flow.IdentityFilter[T] {
	return func(identity *T) bool {
		for _, filter := range filters {
			if filter(identity) {
				return true
			}
		}
		return false
	}
}

// Not returns a filter equivalent to the inverse of the input filter.
func Not[T flow.GenericIdentity](filter flow.IdentityFilter[T]) flow.IdentityFilter[T] {
	return func(identity *T) bool {
		return !filter(identity)
	}
}

// In returns a filter for identities within the input list. For an input identity i,
// the filter returns true if and only if i ∈ list.
// Caution: The filter solely operates on NodeIDs. Other identity fields are not compared.
// This function is just a compact representation of `HasNodeID[T](list.NodeIDs()...)`
// which behaves algorithmically the same way.
func In[T flow.GenericIdentity](list flow.GenericIdentityList[T]) flow.IdentityFilter[T] {
	return HasNodeID[T](list.NodeIDs()...)
}

// HasNodeID returns a filter that returns true for any identity with an ID
// matching any of the inputs.
func HasNodeID[T flow.GenericIdentity](nodeIDs ...flow.Identifier) flow.IdentityFilter[T] {
	lookup := make(map[flow.Identifier]struct{})
	for _, nodeID := range nodeIDs {
		lookup[nodeID] = struct{}{}
	}
	return func(identity *T) bool {
		_, ok := lookup[(*identity).GetNodeID()]
		return ok
	}
}

// HasNetworkingKey returns a filter that returns true for any identity with a
// networking public key matching any of the inputs.
func HasNetworkingKey(keys ...crypto.PublicKey) flow.IdentityFilter[flow.Identity] {
	return func(identity *flow.Identity) bool {
		for _, key := range keys {
			if key.Equals(identity.NetworkPubKey) {
				return true
			}
		}
		return false
	}
}

// HasInitialWeight returns a filter for nodes with non-zero initial weight.
func HasInitialWeight[T flow.GenericIdentity](hasWeight bool) flow.IdentityFilter[T] {
	return func(identity *T) bool {
		return ((*identity).GetInitialWeight() > 0) == hasWeight
	}
}

// HasParticipationStatus is a filter that returns true if the node epoch participation status matches the input.
func HasParticipationStatus(status flow.EpochParticipationStatus) flow.IdentityFilter[flow.Identity] {
	return func(identity *flow.Identity) bool {
		return identity.EpochParticipationStatus == status
	}
}

// HasRole returns a filter for nodes with one of the input roles.
func HasRole[T flow.GenericIdentity](roles ...flow.Role) flow.IdentityFilter[T] {
	lookup := make(map[flow.Role]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(identity *T) bool {
		_, ok := lookup[(*identity).GetRole()]
		return ok
	}
}

// IsValidCurrentEpochParticipant is an identity filter for members of the
// current epoch in good standing.
// Effective it means that node is an active identity in current epoch and has not been ejected.
var IsValidCurrentEpochParticipant = HasParticipationStatus(flow.EpochParticipationStatusActive)

// IsValidCurrentEpochParticipantOrJoining is an identity filter for members of the current epoch or that are going to join in next epoch.
var IsValidCurrentEpochParticipantOrJoining = Or(IsValidCurrentEpochParticipant, HasParticipationStatus(flow.EpochParticipationStatusJoining))

// IsConsensusCommitteeMember is an identity filter for all members of the consensus committee.
// Formally, a Node X is a Consensus Committee Member if and only if X is a consensus node with
// positive initial weight. This is specified by the EpochSetup Event and remains static
// throughout the epoch.
var IsConsensusCommitteeMember = And(
	HasRole[flow.IdentitySkeleton](flow.RoleConsensus),
	HasInitialWeight[flow.IdentitySkeleton](true),
)

// IsVotingConsensusCommitteeMember is an identity filter for all members of
// the consensus committee allowed to vote.
var IsVotingConsensusCommitteeMember = And[flow.Identity](
	HasRole[flow.Identity](flow.RoleConsensus),
	IsValidCurrentEpochParticipant,
)

// IsValidDKGParticipant is an identity filter for all DKG participants. It is
// equivalent to the filter for consensus committee members, as these are
// the same group for now.
var IsValidDKGParticipant = IsConsensusCommitteeMember
