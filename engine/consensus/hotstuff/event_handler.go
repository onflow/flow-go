package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

type EventHandler struct {
	paceMaker             PaceMaker
	voteAggregator        *VoteAggregator
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
	panic("implement me")
}

func (eh *EventHandler) OnLocalTimeout(timeout *types.Timeout) {
	panic("implement me")
}

func (eh *EventHandler) OnBlockRequest(req *types.BlockProposalRequest) {
	panic("implement me")
}
