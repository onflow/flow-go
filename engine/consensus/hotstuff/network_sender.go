package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type NetworkSender interface {
	SendVote(vote *types.Vote, id types.ID)
	BroadcastProposal(b *types.BlockProposal)
	RespondBlockProposalRequest(req *types.BlockProposalRequest, b *types.BlockProposal)
}
