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
// members may be ejected, or have their weight change during an epoch, we ignore these changes.
// For these purposes we use the Replicas and *ByEpoch methods.
//
// When validating proposals, we take into account changes to the committee during the course of
// an epoch. In particular, if a node is ejected, we will immediately reject all future proposals
// from that node. For these purposes we use the DynamicCommittee and *ByBlock methods.

// Replicas defines the consensus committee for the purposes of validating votes, timeouts,
// quorum certificates, and timeout certificates. Any consensus committee member who was
// authorized to contribute to consensus AT THE BEGINNING of the epoch may produce valid
// votes and timeouts for the entire epoch, even if they are later ejected.
// So for validating votes/timeouts we use *ByEpoch methods.
//
// Since the voter committee is considered static over an epoch:
//   - we can query identities by view
//   - we don't need the full block ancestry prior to validating messages
type Replicas interface {

	// LeaderForView returns the identity of the leader for a given view.
	// CAUTION: per liveness requirement of HotStuff, the leader must be fork-independent.
	//          Therefore, a node retains its proposer view slots even if it is slashed.
	//          Its proposal is simply considered invalid, as it is not from a legitimate participant.
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	LeaderForView(view uint64) (flow.Identifier, error)

	// QuorumThresholdForView returns the minimum total weight for a supermajority
	// at the given view. This weight threshold is computed using the total weight
	// of the initial committee and is static over the course of an epoch.
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	QuorumThresholdForView(view uint64) (uint64, error)

	// TimeoutThresholdForView returns the minimum total weight of observed timeout objects
	// required to safely timeout for the given view. This weight threshold is computed
	// using the total weight of the initial committee and is static over the course of
	// an epoch.
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	TimeoutThresholdForView(view uint64) (uint64, error)

	// Self returns our own node identifier.
	// TODO: ultimately, the own identity of the node is necessary for signing.
	//       Ideally, we would move the method for checking whether an Identifier refers to this node to the signer.
	//       This would require some refactoring of EventHandler (postponed to later)
	Self() flow.Identifier

	// DKG returns the DKG info for epoch given by the input view.
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	DKG(view uint64) (DKG, error)

	// IdentitiesByEpoch returns a list of the legitimate HotStuff participants for the epoch
	// given by the input view.
	// The returned list of HotStuff participants:
	//   - contains nodes that are allowed to submit votes or timeouts within the given epoch
	//     (un-ejected, non-zero weight at the beginning of the epoch)
	//   - is ordered in the canonical order
	//   - contains no duplicates.
	//
	// CAUTION: DO NOT use this method for validating block proposals.
	// CAUTION: This method considers epochs outside of Previous, Current, Next, w.r.t. the
	// finalized block, to be unknown. https://github.com/onflow/flow-go/issues/4085
	//
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	//
	IdentitiesByEpoch(view uint64) (flow.IdentitySkeletonList, error)

	// IdentityByEpoch returns the full Identity for specified HotStuff participant.
	// The node must be a legitimate HotStuff participant with NON-ZERO WEIGHT at the specified block.
	// CAUTION: This method considers epochs outside of Previous, Current, Next, w.r.t. the
	// finalized block, to be unknown. https://github.com/onflow/flow-go/issues/4085
	//
	// ERROR conditions:
	//  - model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
	//
	// Returns the following expected errors for invalid inputs:
	//   - model.ErrViewForUnknownEpoch if no epoch containing the given view is known
	//
	IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.IdentitySkeleton, error)
}

// DynamicCommittee extends Replicas to provide the consensus committee for the purposes
// of validating proposals. The proposer committee reflects block-to-block changes in the
// identity table to support immediately rejecting proposals from nodes after they are ejected.
// For validating proposals, we use *ByBlock methods.
//
// Since the proposer committee can change at any block:
//   - we query by block ID
//   - we must have incorporated the full block ancestry prior to validating messages
type DynamicCommittee interface {
	Replicas

	// IdentitiesByBlock returns a list of the legitimate HotStuff participants for the given block.
	// The returned list of HotStuff participants:
	//   - contains nodes that are allowed to submit proposals, votes, and timeouts
	//     (un-ejected, non-zero weight at current block)
	//   - is ordered in the canonical order
	//   - contains no duplicates.
	//
	// ERROR conditions:
	//  - state.ErrUnknownSnapshotReference if the blockID is for an unknown block
	IdentitiesByBlock(blockID flow.Identifier) (flow.IdentityList, error)

	// IdentityByBlock returns the full Identity for specified HotStuff participant.
	// The node must be a legitimate HotStuff participant with NON-ZERO WEIGHT at the specified block.
	// ERROR conditions:
	//  - model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
	//  - state.ErrUnknownSnapshotReference if the blockID is for an unknown block
	IdentityByBlock(blockID flow.Identifier, participantID flow.Identifier) (*flow.Identity, error)
}

// BlockSignerDecoder defines how to convert the ParentSignerIndices field within a
// particular block header to the identifiers of the nodes which signed the block.
type BlockSignerDecoder interface {
	// DecodeSignerIDs decodes the signer indices from the given block header into full node IDs.
	// Note: A block header contains a quorum certificate for its parent, which proves that the
	// consensus committee has reached agreement on validity of parent block. Consequently, the
	// returned IdentifierList contains the consensus participants that signed the parent block.
	// Expected Error returns during normal operations:
	//  - signature.InvalidSignerIndicesError if signer indices included in the header do
	//    not encode a valid subset of the consensus committee
	DecodeSignerIDs(header *flow.Header) (flow.IdentifierList, error)
}

type DKG interface {
	protocol.DKG
}
