package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type EventHandler struct {
	paceMaker             PaceMaker
	voteAggregator        *VoteAggregator
	voter                 *Voter
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
	panic("implement me")
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

func (eh *EventHandler) startNewView() error {
	panic("implement me")
}
