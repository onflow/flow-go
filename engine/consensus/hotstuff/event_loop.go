package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type EventLoop struct {
	eventHandler *EventHandler

	blockproposals chan *types.BlockProposal
	votes          chan *types.Vote
	blockreqs      chan *types.BlockProposalRequest
}

func (el *EventLoop) loop() {
	for {
		timeoutChannel := el.eventHandler.TimeoutChannel()
		select {
		case <-timeoutChannel:
			el.eventHandler.OnLocalTimeout()
		case b := <-el.blockproposals:
			el.eventHandler.OnReceiveBlockProposal(b)
		case v := <-el.votes:
			el.eventHandler.OnReceiveVote(v)
		case req := <-el.blockreqs:
			el.eventHandler.OnBlockRequest(req)
		}
	}
}

func (el *EventLoop) OnReceiveBlockProposal(block *types.BlockProposal) {
	el.blockproposals <- block
}

func (el *EventLoop) OnReceiveVote(vote *types.Vote) {
	el.votes <- vote
}
