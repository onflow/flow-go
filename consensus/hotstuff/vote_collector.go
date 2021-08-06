package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

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

// VoteCollectors holds a map from block ID to vote collector state machine.
// It manages the concurrent access to the map.
type VoteCollectors interface {
	// Get finds a vote collector state machine by the block ID.
	// It returns the vote collector state machine and true if found,
	// It returns nil and false if not found
	GetOrCreate(blockID flow.Identifier) (VoteCollectorStateMachine, bool)

	// ProcessBlock triggers the state transition of the vote colletor state machine from
	// caching status to verifying status.
	ProcessBlock(block model.Proposal) error

	// Prune the vote collectors whose view is below the given view
	PruneByView(view uint64) error
}

// VoteCollectorStateMachine is the state machine for transitioning different status of the vote collector
type VoteCollectorStateMachine interface {
	VoteCollector() VoteCollector
}

// VoteCollector collects votes for the same block, produces QC when enough votes are collected
// VoteCollector takes a callback function to report the event that a QC has been produced.
type VoteCollector interface {
	// AddVote adds a vote to the collector
	// returns true if the vote was added
	// returns false otherwise
	// return error if the signature is invalid
	// When enough votes have been added to produce a QC, the QC will be created asynchronously, and
	// passed to EventLoop through a callback.
	AddVote(vote *model.Vote) (bool, error)

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
	VoteCollector
	BlockSigner
}
