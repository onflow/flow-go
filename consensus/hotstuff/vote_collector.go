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

// VerifyingVoteCollector must implement the same interface as BlockSigner, such that when eventhandler
// asks voter to vote for the block, the voter will ask VoteAggregator(via BlockSigner interface)
// to sign the block, and VoteAggregator will read the VerifyingVoteCollector from the vote collectors
// map and produce the vote.
// Note CachingVoteCollector can't create vote, only VerifyingVoteCollector can
type VerifyingVoteCollector interface {
	BlockSigner
}
