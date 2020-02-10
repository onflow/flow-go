package hotstuff

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type Voter struct {
	signer    Signer
	viewState ViewState
	forks     Forks
	// Flag to turn on/off consensus acts (voting, block production etc)
	isConActor bool
	// Need to keep track of the last view we voted for so we don't double vote accidentally
	lastVotedView uint64
	log           zerolog.Logger
}

func (v *Voter) NewVoter(signer Signer, viewState ViewState, forks Forks, isConActor bool, log zerolog.Logger) *Voter {
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
func (v *Voter) ProduceVoteIfVotable(bp *types.BlockProposal, curView uint64) (*types.Vote, bool) {
	if !v.isConActor {
		log.Debug().Msg("we're not a consensus actor, don't vote")
		return nil, false
	}

	if v.forks.IsSafeBlock(bp) {
		log.Info().Msg("received block is not a safe node, don't vote")
		return nil, false
	}

	if curView != bp.Block.View {
		log.Info().Uint64("view", bp.Block.View).Uint64("curView", curView).
			Msg("received block's view is not our current view, don't vote")
		return nil, false
	}

	if curView <= v.lastVotedView {
		log.Info().Uint64("lastVotedView", v.lastVotedView).Uint64("curView", curView).
			Msg("received block's view is <= lastVotedView, don't vote")
		return nil, false
	}

	vote, err := v.produceVote(bp)
	if err != nil {
		return nil, false
	}
	return vote, true
}

func (v *Voter) produceVote(bp *types.BlockProposal) (*types.Vote, error) {
	myIdentity, err := v.viewState.GetSelfIdentityForBlockID(bp.BlockID())
	if err != nil {
		return nil, fmt.Errorf("can not find self index for block %v: %w", bp.BlockID(), err)
	}
	unsignedVote := types.NewUnsignedVote(bp.Block.View, bp.BlockID())
	sig := v.signer.SignVote(unsignedVote, myIdentity.PubKey)
	vote := unsignedVote.WithSignature(sig)
	return vote, nil
}
