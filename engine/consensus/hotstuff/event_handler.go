package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type EventHandler struct {
	paceMaker      PaceMaker
	voteAggregator *VoteAggregator
	voter          *Voter
	forks          Forks
	validator      *Validator
	blockProducer  *BlockProducer
	viewState      *ViewState
	network        NetworkSender
}

func (eh *EventHandler) OnReceiveBlockHeader(block *types.BlockHeader) {
	panic("implement me")
}

func (eh *EventHandler) OnReceiveVote(vote *types.Vote) {
	blockProposal, found := eh.forks.GetBlock(vote.BlockID[:])
	if found == false {
		eh.voteAggregator.StorePendingVote(vote)
		return
	}
	newQC, err := eh.voteAggregator.StoreVoteAndBuildQC(vote, blockProposal)
	if err != nil {
		//	TODO: handle error
	} else {
		err = eh.forks.AddQC(newQC)
		if err != nil {
			//	TODO: handle events
		}
	}
}

func (eh *EventHandler) TimeoutChannel() <-chan time.Time {
	panic("implement me")
}

func (eh *EventHandler) OnLocalTimeout() {
	panic("implement me")
}

func (eh *EventHandler) startNewView() error {
	panic("implement me")
}
