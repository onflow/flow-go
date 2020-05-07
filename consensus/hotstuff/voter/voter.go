package voter

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// Voter produces votes for the given block
type Voter struct {
	signer        hotstuff.Signer
	forks         hotstuff.ForksReader
	persist       hotstuff.Persister
	lastVotedView uint64 // need to keep track of the last view we voted for so we don't double vote accidentally
}

// New creates a new Voter instance
func New(signer hotstuff.Signer, forks hotstuff.ForksReader, persist hotstuff.Persister, lastVotedView uint64) *Voter {
	return &Voter{
		signer:        signer,
		forks:         forks,
		persist:       persist,
		lastVotedView: lastVotedView,
	}
}

// ProduceVoteIfVotable will make a decision on whether it will vote for the given proposal, the returned
// error indicates whether to vote or not.
// In order to ensure that only a safe node will be voted, Voter will ask Forks whether a vote is a safe node or not.
// The curView is taken as input to ensure Voter will only vote for proposals at current view and prevent double voting.
// This method will only ever _once_ return a `non-nil vote, nil` vote: the very first time it encounters a safe block of the
//  current view to vote for. Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
// (including repeated calls with the initial block we voted for also return `nil, error`).
func (v *Voter) ProduceVoteIfVotable(block *model.Block, curView uint64) (*model.Vote, error) {
	if !v.forks.IsSafeBlock(block) {
		return nil, &model.NoVoteError{Msg: "not safe block"}
	}

	if curView != block.View {
		return nil, &model.NoVoteError{Msg: "not for current view"}
	}

	if curView <= v.lastVotedView {
		return nil, &model.NoVoteError{Msg: "not above the last voted view"}
	}

	vote, err := v.signer.CreateVote(block)
	if err != nil {
		return nil, fmt.Errorf("could not vote for block: %w", err)
	}

	// vote for the current view has been produced, update lastVotedView
	// to prevent from voting for the same view again
	v.lastVotedView = curView
	err = v.persist.PutVoted(curView)
	if err != nil {
		return nil, fmt.Errorf("could not persist last voted: %w", err)
	}

	return vote, nil
}
