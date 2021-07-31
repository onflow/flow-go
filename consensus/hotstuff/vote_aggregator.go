package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// createVote holds the data for signing a block. The constructor of this function should have the block
// info already, taking block again as argument is to ensure it's called with the same block.
// For consensus cluster, the createVote function will create a disposable stateful signer object
// for producing the signature for a vote. After the the signature has been produced, the signer object will
// be discarded.
type createVote func(*model.Block) (*model.Vote, error)

// VoteAggregator verifies and aggregates votes to build QC.
// When enough votes have been collected, it builds a QC and send it to the EventLoop
// VoteAggregator also detects protocol violation, including invalid votes, double voting etc, and
// notifies a HotStuff consumer for slashing.
type VoteAggregator interface {

	// AddVote verifies and aggregates a vote.
	// The voting block could either be known or unknown.
	// If the voting block is unknown, the vote won't be processed until AddBlock is called with the block.
	// This method can be called concurrently, votes will be queued and processed asynchronously.
	AddVote(vote *model.Vote) error

	// AddBlock notifies the VoteAggregator about a known block so that it can start processing
	// pending votes whose block was unknown.
	// It also verifies the proposer vote of a block, and return whether the proposer signature is valid.
	AddBlock(block *model.Block) (bool, error)

	// GetVoteCreator returns a createVote function for a given block
	// The caller must ensure the block is a known block by calling AddBlock before.
	GetVoteCreator(block *model.Block) (createVote, error)

	// InvalidBlock notifies the VoteAggregator about an invalid block, so that it can process votes for the invalid
	// block and slash the voters.
	InvalidBlock(block *model.Block)

	// PruneByView will remove any data held for the provided view.
	PruneByView(view uint64)
}
