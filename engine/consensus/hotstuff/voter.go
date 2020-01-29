package hotstuff

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/rs/zerolog"
)

type Voter struct {
	signer    Signer
	viewState *ViewState
	forks     Forks
	// Flag to turn on/off consensus acts (voting, block production etc)
	isConActor bool
	// Need to keep track of the last view we voted for so we don't double vote accidentally
	lastVotedView uint64
	log           zerolog.Logger
}

func (v *Voter) NewVoter(signer Signer, viewState *ViewState, forks Forks, isConActor bool, log zerolog.Logger) *Voter {
	return &Voter{
		signer:        signer,
		viewState:     viewState,
		forks:         forks,
		isConActor:    isConActor,
		lastVotedView: 0,
		log:           log,
	}
}

// ProduceVoteIfVotable will make a decision on whether it will vote for the given proposal, the returned
// boolean indicates whether to vote or not.
// In order to ensure that only a safe node will be voted, Voter will ask Forks whether a vote is a safe node or not.
// The curView is taken as input to ensure Voter will only vote for proposals at current view and prevent double voting.
// This method will only ever _once_ return a `non-nil, true` vote: the very first time it encounters a safe block of the
//  current view to vote for. Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
// (including repeated calls with the initial block we voted for also return `nil, false`).
func (v *Voter) ProduceVoteIfVotable(bp *types.BlockProposal, curView uint64) (*types.Vote, error) {
	if !v.isConActor {
		return nil, fmt.Errorf("we're not a consensus actor, don't vote")
	}

	if v.forks.IsSafeBlock(bp) {
		return nil, fmt.Errorf("received block is not a safe block, don't vote")
	}

	if curView != bp.Block.View {
		return nil, fmt.Errorf("received block's view is not our current view, don't vote")
	}

	if curView <= v.lastVotedView {
		return nil, fmt.Errorf("received block's view is <= lastVotedView, don't vote")
	}

	vote, err := v.produceVote(bp)
	if err != nil {
		return nil, err
	}
	return vote, nil
}

func (v *Voter) produceVote(bp *types.BlockProposal) (*types.Vote, error) {
	signerIdx, err := v.viewState.GetSelfIdxForBlockID(bp.BlockMRH())
	if err != nil {
		return nil, err
	}
	unsignedVote := types.NewUnsignedVote(bp.Block.View, bp.Block.BlockMRH())
	sig := v.signer.SignVote(unsignedVote, signerIdx)
	vote := types.NewVote(unsignedVote, sig)

	return vote, nil
}
