package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// OnQCCreated is a callback which will be used by VoteCollector to submit a QC when it's able to create it
type OnQCCreated func(*flow.QuorumCertificate)

// VoteCollectorStatus indicates the VoteCollector's status
// It has three different status.
type VoteCollectorStatus int

const (
	// VoteCollectorStatusCaching is for the status when the block has not been received.
	// The vote collector in this status will cache all the votes without verifying them
	VoteCollectorStatusCaching = iota

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

type VoteCollector interface {
	VoteCollectorState
	// ChangeProcessingStatus changes the VoteCollector's internal processing
	// status. The operation is implemented as an atomic compare-and-swap, i.e. the
	// state transition is only executed if VoteCollector's internal state is
	// equal to `expectedValue`. The return indicates whether the state was updated.
	// The implementation only allows the transitions
	//         CachingVotes   -> VerifyingVotes
	//         CachingVotes   -> Invalid
	//         VerifyingVotes -> Invalid
	// Error returns:
	// * nil if the state transition was successfully executed
	// * ErrDifferentCollectorState if the VoteCollector's state is different than expectedCurrentStatus
	// * ErrInvalidCollectorStateTransition if the given state transition is impossible
	// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
	ChangeProcessingStatus(expectedValue, newValue VoteCollectorStatus) error

	// ProcessBlock performs validation of block signature and processes block with respected collector.
	// Calling this function will mark conflicting collector as stale and change state of valid collectors
	// It returns nil if the block is valid.
	// It returns model.InvalidBlockError if block is invalid.
	// It returns other error if there is exception processing the block.
	ProcessBlock(block *model.Proposal) error
}

// VoteCollectorState collects votes for the same block, produces QC when enough votes are collected
// VoteCollectorState takes a callback function to report the event that a QC has been produced.
type VoteCollectorState interface {
	// AddVote adds a vote to the collector
	// return error if the signature is invalid
	// When enough votes have been added to produce a QC, the QC will be created asynchronously, and
	// passed to EventLoop through a callback.
	AddVote(vote *model.Vote) error

	// BlockID returns the block ID that this instance is collecting votes for.
	// This method is useful when adding the newly created vote collector to vote collectors map.
	BlockID() flow.Identifier

	// Status returns the status of the vote collector
	Status() VoteCollectorStatus
}

// VerifyingVoteCollector is a VoteCollector and also implement the same interface as BlockSigner, so that
// when the voter ask VoteAggregator(via BlockSigner interface)
// to sign the block, and VoteAggregator will read the VerifyingVoteCollector from the vote collectors
// map and produce the vote.
// Note CachingVoteCollector can't create vote, only VerifyingVoteCollector can
type VerifyingVoteCollector interface {
	VoteCollectorState
	BlockSigner
}
