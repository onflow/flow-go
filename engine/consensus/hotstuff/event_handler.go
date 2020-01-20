package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/voteaggregator"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type EventHandler struct {
	paceMaker             PaceMaker
	voteAggregator        *voteaggregator.VoteAggregator
	voter                 *Voter
	missingBlockRequester *MissingBlockRequester
	forks                 Forks
	validator             *Validator
	blockProposalProducer BlockProposalProducer
	viewState             ViewState
	network               NetworkSender
}

func (eh *EventHandler) OnReceiveBlockProposal(block *types.BlockProposal) {
	panic("implement me")
}

func (eh *EventHandler) OnReceiveVote(vote *types.Vote) {
	blockProposal, found := eh.forks.GetBlock(vote.View, vote.BlockMRH)
	if found == false {
		eh.voteAggregator.StorePendingVote(vote)
		eh.missingBlockRequester.FetchMissingBlock(vote.View, vote.BlockMRH)
		return
	}
	if newQC, ok := eh.voteAggregator.StoreVoteAndBuildQC(vote, blockProposal); ok == true {
		if err := eh.forks.AddQC(newQC); err != nil {
			//	TODO: handle error
		}
	} else {
		//	TODO: handle events
	}
}

func (eh *EventHandler) TimeoutChannel() <-chan time.Time {
	panic("implement me")
}

func (eh *EventHandler) OnLocalTimeout() {
	panic("implement me")
}

func (eh *EventHandler) OnBlockRequest(req *types.BlockProposalRequest) {
	panic("implement me")
}
