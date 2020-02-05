package genericstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

type Communicator interface {
	SendVote(vote *Vote, to flow.Identifier) error
	BroadcastProposal(proposal *Proposal) error
	BroadcastCommit(commit *Commit) error
}

type Finalizer interface {
	Finalize(blockID flow.Identifier)
}
