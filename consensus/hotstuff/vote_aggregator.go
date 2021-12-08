package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
)

// BlockSigner abstracts the implementation of how a signature of a block or a vote is produced
// and stored in a stateful crypto object for aggregation.
// The VoteAggregator implements both the VoteAggregator interface and the BlockSigner interface so that
// the EventHandler could use the VoteAggregator interface to sign a Block, and Voter/BlockProducer can use
// the BlockSigner interface to create vote.
// When `CreateVote` is called, it internally creates stateful VoteCollector object, which also has the ability
// to verify the block and generate the vote signature.
// The created vote collector will be added to the vote collectors map. These
// implementation details are abstracted to Voter/BlockProducer.
type BlockSigner interface {
	// CreateVote returns a vote for the given block.
	// It returns:
	//  - (vote, nil) if vote is created
	//  - (nil , module.InvalidBlockError) if the block is invalid.
	CreateVote(*model.Block) (*model.Vote, error)
}

// VoteAggregator verifies and aggregates votes to build QC.
// When enough votes have been collected, it builds a QC and send it to the EventLoop
// VoteAggregator also detects protocol violation, including invalid votes, double voting etc, and
// notifies a HotStuff consumer for slashing.
type VoteAggregator interface {
	module.ReadyDoneAware
	module.Startable

	// AddVote verifies and aggregates a vote.
	// The voting block could either be known or unknown.
	// If the voting block is unknown, the vote won't be processed until AddBlock is called with the block.
	// This method can be called concurrently, votes will be queued and processed asynchronously.
	AddVote(vote *model.Vote)

	// AddBlock notifies the VoteAggregator that it should start processing votes for the given block.
	// AddBlock is a _synchronous_ call (logic is executed by the calling go routine). It also verifies
	// validity of the proposer's vote for its own block.
	// Expected error returns during normal operations:
	// * model.InvalidBlockError if the proposer's vote for its own block is invalid
	// * mempool.DecreasingPruningHeightError if the block's view has already been pruned
	AddBlock(block *model.Proposal) error

	// InvalidBlock notifies the VoteAggregator about an invalid proposal, so that it
	// can process votes for the invalid block and slash the voters. Expected error
	// returns during normal operations:
	// * mempool.DecreasingPruningHeightError if proposal's view has already been pruned
	InvalidBlock(block *model.Proposal) error

	// PruneUpToView deletes all votes _below_ to the given view, as well as
	// related indices. We only retain and process whose view is equal or larger
	// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
	// previous value, the previous value is kept and the method call is a NoOp.
	PruneUpToView(view uint64)
}
