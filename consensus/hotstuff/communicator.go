package hotstuff

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Communicator is the component that allows the HotStuff core algorithm to
// communicate with the other actors of the consensus process.
type Communicator interface {

	// SendVote sends a vote for the given parameters to the specified recipient.
	SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error

	// BroadcastProposal broadcasts the given block proposal to all actors of
	// the consensus process.
	BroadcastProposal(proposal *flow.Header) error
}
