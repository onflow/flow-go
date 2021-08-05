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
	// ToDo: document sentinel errors expected during normal operation
	AddVote(vote *model.Vote) (bool, error)

	// BlockID returns the block ID that this instance is collecting votes for.
	// This method is useful when adding the newly created vote collector to vote collectors map.
	BlockID() flow.Identifier

	// Block returns the block that this instance is collecting votes for.
	Block() *model.Block
}
