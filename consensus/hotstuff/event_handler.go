package hotstuff

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// EventHandler runs a state machine to process proposals, votes and local timeouts.
type EventHandler interface {

	// OnReceiveVote processes a vote received from another HotStuff consensus
	// participant.
	OnReceiveVote(vote *model.Vote) error

	// OnReceiveProposal processes a block proposal received fro another HotStuff
	// consensus participant.
	OnReceiveProposal(proposal *model.Proposal) error

	// OnLocalTimeout will check if there was a local timeout.
	OnLocalTimeout() error

	// TimeoutChannel returs a channel that sends a signal on timeout.
	TimeoutChannel() <-chan time.Time

	// Start will start the event handler.
	Start() error
}
