package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/HotStuff/components/voteaggregator"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"time"
)

type EventHandler struct {
	paceMaker             *PaceMaker
	voteAggregator        *voteaggregator.VoteAggregator
	voter                 *Voter
	missingBlockRequester *MissingBlockRequester
	reactor               Crown
	validator             *Validator
	blockProposalProducer BlockProposalProducer
	viewState             ViewState
	network               NetworkSender
}

func (eh *EventHandler) OnReceiveBlockProposal(block *types.BlockProposal) {
	panic("implement me")
}

func (eh *EventHandler) OnReceiveVote(vote *types.Vote) {
	blockProposal, found := eh.reactor.GetBlock(vote.View, vote.BlockMRH)
	if found == false {
		if err := eh.voteAggregator.StorePendingVote(vote); err != nil {
			// TODO: handle error
			return
		}
		eh.missingBlockRequester.FetchMissingBlock(vote.View, vote.BlockMRH)
		return
	}
	if newQC, err := eh.voteAggregator.StoreVoteAndBuildQC(vote, blockProposal); err == nil {
		if err := eh.reactor.AddQC(newQC); err != nil {
			//	TODO: handle error
		}
	} else {
		//	TODO: handle error
	}
}

func (eh *EventHandler) OnLocalTimeout() {
	panic("implement me")
}

func (eh *EventHandler) OnBlockRequest(req *types.BlockProposalRequest) {
	panic("implement me")
}

func (eh *EventHandler) TimeoutChannel() <-chan time.Time {
	panic("implement me")
}
