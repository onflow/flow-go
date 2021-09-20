package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
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
// TODO: rename to remove V2 when replacing V1
type VoteAggregatorV2 interface {
	// AddVote verifies and aggregates a vote.
	// The voting block could either be known or unknown.
	// If the voting block is unknown, the vote won't be processed until AddBlock is called with the block.
	// This method can be called concurrently, votes will be queued and processed asynchronously.
	// No errors are expected during normal operations, any error should be threatened as exception
	AddVote(vote *model.Vote) error

	// AddBlock notifies the VoteAggregator about a known block so that it can start processing
	// pending votes whose block was unknown.
	// It also verifies the proposer vote of a block, and return whether the proposer signature is valid.
	// Expected error returns during normal operations:
	// * model.InvalidBlockError if the block is invalid
	AddBlock(block *model.Proposal) error

	// InvalidBlock notifies the VoteAggregator about an invalid proposal, so that it
	// can process votes for the invalid block and slash the voters. Expected error
	// returns during normal operations:
	// * mempool.DecreasingPruningHeightError if proposal's view has already been pruned
	InvalidBlock(block *model.Proposal) error

	// PruneUpToView prunes the vote collectors whose view is below the given view.
	// If `view` is smaller than the previous value, the previous value is kept
	// and no operations will be made. Sets highest pruned view which will be used
	// to drop stale proposals and votes.
	PruneUpToView(view uint64)
}
