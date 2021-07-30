package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type ProcessingStatus int

const (
	CachingVotes ProcessingStatus = iota
	VerifyingVotes
	Invalid
)

func (ps ProcessingStatus) String() string {
	names := [...]string{"CachingVotes", "VerifyingVotes", "Invalid"}
	if ps < CachingVotes || ps > Invalid {
		return "UNKNOWN"
	}
	return names[ps]
}

// OnQCCreated is callback which should be called by VoteCollector whenever
// it produces a QC.
type OnQCCreated func(*flow.QuorumCertificate)

// VoteCollectorState collects votes for the same block, produces QC when enough votes are collected
type VoteCollectorState interface {
	// AddVote adds a vote to the collector
	// if vote collector was able to create a QC, it will be returned together will bool value indicating success
	// return error if the signature is invalid
	// When enough votes have been added to produce a QC, the QC will be created asynchronously, and
	// passed to EventLoop through a callback.
	AddVote(vote *model.Vote) error

	// VoteCreator returns a function that is able to create a vote for a given block
	// The returned CreateVote function holds a stateful crypto signer object that is able to
	// verify and store a signature share from other nodes, as well as produce a signature for
	// our own vote. It is used by Voter to create our own vote.
	VoteCreator() CreateVote

	// BlockID returns the block ID that the collector is collecting votes for.
	// This method is useful when adding the newly created vote collector to vote collectors map.
	BlockID() flow.Identifier

	// ProcessingStatus returns the VoteCollector's  ProcessingStatus
	ProcessingStatus() ProcessingStatus
}

type VoteCollector interface {
	VoteCollectorState
	// ChangeProcessingStatus changes the VoteCollector's internal processing
	// status. The operation is implemented as an atomic compare-and-swap, i.e. the
	// state transition is only executed if VoteCollector's internal state is
	// equal to `expectedValue`. The return indicates whether the state was updated.
	// The implementation only allows the transitions
	//         CachingVotes -> VerifyingVotes
	//    and                  VerifyingVotes -> Invalid
	// Error returns:
	// * nil if the state transition was successfully executed
	// * ErrDifferentCollectorState if the VoteCollector's state is different than expectedCurrentStatus
	// * ErrInvalidCollectorStateTransition if the given state transition is impossible
	// * all other errors are unexpected and potential symptoms of internal bugs or state corruption (fatal)
	ChangeProcessingStatus(expectedValue, newValue ProcessingStatus) error
}
