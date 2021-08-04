package voteaggregator

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/rs/zerolog"
)

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
type VoteAggregator struct {
	log        zerolog.Logger // used to log relevant actions with context
	queue      hotstuff.VotesQueue
	unit       *engine.Unit
	collectors VoteCollectors
	// notifier              hotstuff.Consumer
	// committee             hotstuff.Committee
	// voteValidator         hotstuff.Validator
	// signer                hotstuff.SignerVerifier
	// highestPrunedView     uint64
	// pendingVotes          *PendingVotes                               // keeps track of votes whose blocks can not be found
	// viewToBlockIDSet      map[uint64]map[flow.Identifier]struct{}     // for pruning
	// viewToVoteID          map[uint64]map[flow.Identifier]*model.Vote  // for detecting double voting
	// createdQC             map[flow.Identifier]*flow.QuorumCertificate // keeps track of QCs that have been made for blocks
	// blockIDToVotingStatus map[flow.Identifier]*VotingStatus           // keeps track of accumulated votes and stakes for blocks
	// proposerVotes         map[flow.Identifier]*model.Vote             // holds the votes of block proposers, so we can avoid passing around proposals everywhere
}

func NewVoteAggregator() *VoteAggregator {
	return &VoteAggregator{
		queue:      newQueue(),
		unit:       engine.NewUnit(),
		collectors: NewVoteCollectors(),
	}
}

func newQueue() hotstuff.VotesQueue {
	// TO implement
	return nil
}

func (v *VoteAggregator) Run(workers int) {
	v.queue.Consume(workers, v.processVote, v.unit.Quit())
}

// AddVote verifies and aggregates a vote.
// The voting block could either be known or unknown.
// If the voting block is unknown, the vote won't be processed until AddBlock is called with the block.
// This method can be called concurrently, votes will be queued and processed asynchronously.
func (v *VoteAggregator) AddVote(vote *model.Vote) error {
	added := v.queue.Push(vote)
	return nil
}

// AddBlock notifies the VoteAggregator about a known block so that it can start processing
// pending votes whose block was unknown.
// It also verifies the proposer vote of a block, and return whether the proposer signature is valid.
func (v *VoteAggregator) AddBlock(block *model.Block) (bool, error) {
}

// GetVoteCreator returns a createVote function for a given block
// The caller must ensure the block is a known block by calling AddBlock before.
func (v *VoteAggregator) GetVoteCreator(block *model.Block) (createVote, error) {
}

// InvalidBlock notifies the VoteAggregator about an invalid block, so that it can process votes for the invalid
// block and slash the voters.
func (v *VoteAggregator) InvalidBlock(block *model.Block) {
}

// PruneByView will remove any data held for the provided view.
func (v *VoteAggregator) PruneByView(view uint64) {
	v.queue.PruneByView(view)
}

func (v *VoteAggregator) processVote(vote *model.Vote) error {
	collector, created := v.collectors.GetOrCreate(vote.BlockID)
	added, err := collector.AddVote(vote)
	if engine.IsInvalidInputError(err) {
		return fmt.Errorf("invalid vote: %w", err)
	}

	v.log.With().Bool("collector created", created).Bool("vote added", added).Msg("vote processed")
	return nil
}
