package hotstuff

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteConsumer consumes all votes for one specific view. It is registered with
// the `VoteCollector` for the respective view. Upon registration, the
// `VoteCollector` feeds votes into the consumer in the order they are received
// (already cached votes as well as votes received in the future). Only votes
// that pass de-duplication and equivocation detection are passed on. CAUTION,
// VoteConsumer implementations must be
//   - NON-BLOCKING and consume the votes without noteworthy delay, and
//   - CONCURRENCY SAFE
type VoteConsumer func(vote *model.Vote)

// OnQCCreated is a callback which will be used by VoteCollector to submit a QC when it's able to create it
type OnQCCreated func(*flow.QuorumCertificate)

// VoteCollectorStatus indicates the VoteCollector's status
// It has three different status.
type VoteCollectorStatus int

const (
	// VoteCollectorStatusCaching is for the status when the block has not been received.
	// The vote collector in this status will cache all the votes without verifying them
	VoteCollectorStatusCaching VoteCollectorStatus = iota

	// VoteCollectorStatusVerifying is for the status when the block has been received,
	// and is able to process all votes for it.
	VoteCollectorStatusVerifying
)

// VoteCollector collects votes for the same block, produces QC when enough votes are collected
// VoteCollector takes a callback function to report the event that a QC has been produced.
var collectorStatusNames = [...]string{"VoteCollectorStatusCaching",
	"VoteCollectorStatusVerifying",
	"VoteCollectorStatusInvalid"}

func (ps VoteCollectorStatus) String() string {
	if ps < 0 || int(ps) > len(collectorStatusNames) {
		return "UNKNOWN"
	}
	return collectorStatusNames[ps]
}

// VoteCollector collects all votes for a specified view. On the happy path, it generates a QC when enough votes have been
// collected. The [VoteCollector] internally delegates the vote-format specific processing to the [hotstuff.VoteProcessor].
//
// External REQUIREMENT:
//   - The [VoteCollector] must receive only blocks that passed the Compliance Layer, i.e. blocks that are valid. Otherwise,
//     The [VoteCollector] might produce QCs for invalid blocks, should a byzantine supermajority exist in the committee
//     producing such votes. This is very unlikely in practice, but the probability is still too large to ignore for Flow's
//     Cluster Consensus. More generally, byzantine supermajorities are plausible in architectures, where small consensus
//     committees are sampled from larger populations of nodes, with byzantine stake close to 1/3.
//     If given an invalid proposal and in the present of a byzantine supermajority, [VoteCollector] might produce a
//     cryptographically valid QC for an invalid block, thereby actively aiding byzantine actors in the committee.
//
// BFT NOTES:
// The stack of [VoteCollector] plus [hotstuff.VoteProcessor] is resilient (safe and live) against any vote-driven attacks:
//   - The [VoteCollector] guarantees liveness, by shielding the [hotstuff.VoteProcessor] from resource exhaustion attacks via
//     repeated, potentially equivocating stand-alone votes and/or votes embedded into proposals. Checks should be very fast
//     (no cryptograph involved) and hence assumed to not become a bottleneck compared to the consumed networking bandwidth
//     and decoding work in case this node is under attack.
//     As the first layer of defense, the [hotstuff.VoteProcessor] detects and rejects duplicates and equivocations.
//     [VoteCollector] reliably reports the first equivocation attempt; repeated reports about the same offending node may be
//     dropped without loss of generality.
//   - The [hotstuff.VoteProcessor] guarantees safety of the concurrent QC generation logic, being resilient against arbitrary
//     byzantine inputs, including cryptographic validity checks.
//   - Proposal equivocation is handled and reported by [hotstuff.Forks], so we don't have to do anything here. [VoteCollector]
//     can ignore anything but the first (valid) proposal for a view.
type VoteCollector interface {
	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collector as stale and change state of valid collectors
	// It returns nil if the block is valid.
	// It returns [model.InvalidProposalError] if block is invalid.
	// It returns other error if there is exception processing the block.
	ProcessBlock(block *model.SignedProposal) error

	// AddVote adds a vote to the vote collector. The vote must be for the `VoteCollector`'s view (otherwise,
	// an exception is returned). When enough votes have been added to produce a QC, the QC will be created
	// asynchronously, and passed to EventLoop through a callback.
	// All byzantine edge cases are handled internally via callbacks to notifier.
	// Under normal execution only exceptions are propagated to caller.
	AddVote(vote *model.Vote) error

	// RegisterVoteConsumer registers a VoteConsumer. Upon registration, the collector
	// feeds all cached votes into the consumer in the order they arrived.
	// CAUTION, VoteConsumer implementations must be
	//  * NON-BLOCKING and consume the votes without noteworthy delay, and
	//  * CONCURRENCY SAFE
	RegisterVoteConsumer(consumer VoteConsumer)

	// View returns the view that this instance is collecting votes for.
	// This method is useful when adding the newly created vote collector to vote collectors map.
	View() uint64

	// Status returns the status of the vote collector
	Status() VoteCollectorStatus
}

// VoteProcessor processes votes. It implements the vote-format specific processing logic.
// Depending on their implementation, a VoteProcessor might drop votes or attempt to construct a QC.
//
// BFT NOTES:
//   - The [VoteProcessor] is entirely resilient to repeated, invalid and/or equivocating votes, thereby providing
//     safety against vote-driven attacks.
//
// ATTENTION BFT LIMITATION:
// The [VoteProcessor]'s primary responsibility is to construct a valid QC. It reliably detects invalid votes - if they reach
// the [VoteProcessor], i.e. aren't already rejected and flagged as an equivocation attack by the [VoteCollector]. The [VoteProcessor]
// responds with dedicated sentinel errors when it rejects a vote (e.g., due to equivocation or invalidity). However, the [VoteProcessor]
// is not designed to reliably detect all equivocation attempts.
type VoteProcessor interface {
	// Process performs processing of single vote. This function is safe to call from multiple goroutines.
	//
	// Expected error returns during normal operations:
	//   - [VoteForIncompatibleBlockError] if vote is for incompatible block
	//   - [VoteForIncompatibleViewError] if vote is for incompatible view
	//   - [model.InvalidVoteError] if vote has invalid signature
	//   - [model.DuplicatedSignerError] if the same vote from the same signer has been already added
	//
	// All other errors should be treated as exceptions.
	Process(vote *model.Vote) error

	// Status returns the status of the vote processor
	Status() VoteCollectorStatus
}

// VerifyingVoteProcessor is a VoteProcessor that attempts to construct a QC for a specific block
// (provided at construction time).
//
// IMPORTANT: The VerifyingVoteProcessor provides the final defense against any vote-equivocation attacks
// for its specific block. These attacks typically aim at multiple votes from the same node being counted
// towards the supermajority threshold. The VerifyingVoteProcessor must withstand attacks by the
// leader concurrently utilizing stand-alone votes and votes embedded into the proposal.
type VerifyingVoteProcessor interface {
	VoteProcessor

	// Block returns which block that will be used to collector votes for. Transition to VerifyingVoteCollector can occur only
	// when we have received block proposal so this information has to be available.
	Block() *model.Block
}

// VoteProcessorFactory is a factory that can be used to create a verifying vote processors for a specific proposal.
// Depending on factory implementation it will return processors for consensus or collection clusters
type VoteProcessorFactory interface {
	// Create instantiates a VerifyingVoteProcessor for processing votes for a specific proposal.
	// Caller can be sure that proposal vote was successfully verified and processed.
	// Expected error returns during normal operations:
	// * [model.InvalidProposalError] - proposal has invalid proposer vote
	Create(log zerolog.Logger, proposal *model.SignedProposal) (VerifyingVoteProcessor, error)
}
