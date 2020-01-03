package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type Network interface {
	SendVote(vote *types.Vote, identityIdx uint32)
	BroadcastProposal(b *types.BlockProposal)
	RespondBlockProposalRequest(req *types.BlockProposalRequest, b *types.BlockProposal)
}
