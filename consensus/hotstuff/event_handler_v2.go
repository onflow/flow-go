package hotstuff

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// EventHandler runs a state machine to process proposals, QC and local timeouts.
// TODO: rename to remove V2 when replacing V1
type EventHandlerV2 interface {

	// OnQCConstructed processes a valid qc constructed by internal vote aggregator.
	OnQCConstructed(qc *flow.QuorumCertificate) error

	// OnReceiveProposal processes a block proposal received from another HotStuff
	// consensus participant.
	OnReceiveProposal(proposal *model.Proposal) error

	// OnLocalTimeout will check if there was a local timeout.
	OnLocalTimeout() error

	// TimeoutChannel returs a channel that sends a signal on timeout.
	TimeoutChannel() <-chan time.Time

	// Start starts the event handler.
	Start() error
}
