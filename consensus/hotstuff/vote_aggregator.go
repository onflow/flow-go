package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// BlockSigner provides the privileged ability to sign a block, i.e. create a consensus vote for the block.
// ToDo: move the following documentation to the implementation
// The VoteAggregator implements both the VoteAggregator interface and the BlockSigner interface.
// When `CreateVote` is called, it internally creates stateful VoteCollector object, which also has the ability
// to generate the vote signature. Thereafter, the VoteCollector object can be discarded.
type BlockSigner interface {
	// CreateVote returns a vote for the given block.
	// ToDo: document sentinel errors expected during normal operation
	CreateVote(*model.Block) (*model.Vote, error)
}

// VoteAggregator verifies and aggregates votes to build QC.
// When enough votes have been collected, it builds a QC and send it to the EventLoop
// VoteAggregator also detects protocol violation, including invalid votes, double voting etc, and
// notifies a HotStuff consumer for slashing.
type VoteAggregator interface {

	// AddVote verifies and aggregates a vote.
	// The voting block could either be known or unknown.
	// If the voting block is unknown, the vote won't be processed until AddBlock is called with the block.
	// This method can be called concurrently, votes will be queued and processed asynchronously.
	// ToDo: document sentinel errors expected during normal operation
	AddVote(vote *model.Vote) error

	// AddBlock notifies the VoteAggregator about a known block so that it can start processing
	// pending votes whose block was unknown.
	// It also verifies the proposer vote of a block, and return whether the proposer signature is valid.
	// ToDo: document sentinel errors expected during normal operation
	AddBlock(block *model.Proposal) (bool, error)

	// InvalidBlock notifies the VoteAggregator about an invalid block, so that it can process votes for the invalid
	// block and slash the voters.
	InvalidBlock(block *model.Block)

	// PruneByView will remove any data held for the provided view.
	PruneByView(view uint64)
}
