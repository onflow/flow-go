package hotstuff

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

type NetworkSender interface {
	SendVote(vote *types.Vote, id flow.Identity)
	BroadcastProposal(b *types.BlockProposal)
	RespondBlockProposalRequest(req *types.BlockProposalRequest, b *types.BlockProposal)
}
