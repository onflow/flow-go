package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// VoteConsumer consumes all votes for one specific view. It is registered with
// the `VoteCollector` for the respective view. Upon registration, the
// `VoteCollector` feeds votes into the consumer in the order they are received
// (already cached votes as well as votes received in the future). Only votes
// that pass de-duplication and equivocation detection are passed on. CAUTION,
// VoteConsumer implementations must be
//  * NON-BLOCKING and consume the votes without noteworthy delay, and
//  * CONCURRENCY SAFE
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

	// VoteCollectorStatusInvalid is for the status when the block has been verified and
	// is invalid. All votes to this block will be collected to slash the voter.
	VoteCollectorStatusInvalid
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

// VoteCollector collects all votes for a specified view. On the happy path, it
// generates a QC when enough votes have been collected.
// The VoteCollector internally delegates the vote-format specific processing
// to the VoteProcessor.
type VoteCollector interface {
	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collector as stale and change state of valid collectors
	// It returns nil if the block is valid.
	// It returns model.InvalidBlockError if block is invalid.
	// It returns other error if there is exception processing the block.
	ProcessBlock(block *model.Proposal) error

	// AddVote adds a vote to the collector
	// return error if the signature is invalid
	// When enough votes have been added to produce a QC, the QC will be created asynchronously, and
	// passed to EventLoop through a callback.
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
type VoteProcessor interface {
	// Process performs processing of single vote. This function is safe to call from multiple goroutines.
	// Expected error returns during normal operations:
	// * VoteForIncompatibleBlockError - submitted vote for incompatible block
	// * VoteForIncompatibleViewError - submitted vote for incompatible view
	// * model.InvalidVoteError - submitted vote with invalid signature
	// All other errors should be treated as exceptions.
	Process(vote *model.Vote) error

	// Status returns the status of the vote processor
	Status() VoteCollectorStatus
}

// VerifyingVoteProcessor is a VoteProcessor that attempts to construct a QC for the given block.
type VerifyingVoteProcessor interface {
	VoteProcessor

	// Block returns which block that will be used to collector votes for. Transition to VerifyingVoteCollector can occur only
	// when we have received block proposal so this information has to be available.
	Block() *model.Block
}

// VoteProcessorFactory is a factory that can be used to create verifying vote processors for current proposal.
// Depending on factory implementation it will return processors for consensus or collection clusters
type VoteProcessorFactory interface {
	// Create creates instance of VerifyingVoteProcessor for processing votes for current proposal.
	// After constructing VerifyingVoteProcessor validity of proposer vote is checked.
	// Caller can be sure that proposal vote was verified and processed.
	// Expected error returns during normal operations:
	// * model.InvalidBlockError - proposal has invalid proposer vote
	Create(proposal *model.Proposal) (VerifyingVoteProcessor, error)
}
