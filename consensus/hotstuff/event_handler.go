package hotstuff

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// EventHandler runs a state machine to process proposals, QC and local timeouts.
type EventHandler interface {

	// OnReceiveQC processes a valid qc received from internal vote aggregator.
	OnReceiveQC(qc *flow.QuorumCertificate) error

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
