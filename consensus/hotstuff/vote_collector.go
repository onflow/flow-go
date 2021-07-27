package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
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

	// VoteCreator returns a function that is able to create a vote for a given block
	// The returned createVote function holds a stateful crypto signer object that is able to
	// verify and store a signature share from other nodes, as well as produce a signature for
	// our own vote. It is used by Voter to create our own vote.
	VoteCreator() createVote

	// BlockID returns the block ID that the collector is collecting votes for.
	// This method is useful when adding the newly created vote collector to vote collectors map.
	BlockID() flow.Identifier
}
