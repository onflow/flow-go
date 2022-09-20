package consensus

import (
	"github.com/onflow/flow-go/model/messages"
)

// ProposalProvider provides proposals created by this node to non-consensus nodes.
type ProposalProvider interface {
	// ProvideProposal asynchronously submits our proposal to all non-consensus nodes.
	ProvideProposal(proposal *messages.BlockProposal)
}
