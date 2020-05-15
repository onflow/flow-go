package hotstuff

import (
	"time"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

type EventHandler interface {

	// Start will start the event handler.
	Start() error

	// StartNewView makes the event handler transition to the next view.
	StartNewView() error

	// OnReceiveVote processes a vote received from another HotStuff consensus
	// participant.
	OnReceiveVote(vote *model.Vote) error

	// OnReceiveProposal processes ablock proposal received fro another HotStuff
	// consensus participant.
	OnReceiveProposal(proposal *model.Proposal) error

	// OnLocalTimeout will check if there was a local timeout.
	OnLocalTimeout() error

	// TimeoutChannel returs a channel that sends a signal on timeout.
	TimeoutChannel() <-chan time.Time
}
