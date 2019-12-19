package verifier

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	log   zerolog.Logger  // used to log relevant actions
	con   network.Conduit // used for inter-node communication within the network
	me    module.Local    // used to access local node information
	state protocol.State  // used to access the  protocol state
}

// New creates and returns a new instance of a verifier engine
func New(loger zerolog.Logger, net module.Network, state protocol.State, me module.Local) (*Engine, error) {
	e := &Engine{
		log:   loger,
		state: state,
		me:    me,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.VerificationVerifier, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Submit is exposed to the internal engines of the node, and not the network
// It contains an internal call to the Process method of the engine,
// It does not return an error, rather it logs it internally
func (e *Engine) Submit(event interface{}) {
	// process the event with our own ID to indicate it's local
	err := e.Process(e.me.NodeID(), event)
	if err != nil {
		e.log.Error().
			Err(err).
			Msg("could not process local event")
	}
}

// Process is exposed to the network.
// It receives and submits an event to the verifier engine for processing.
// It returns an error so the verifier engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) Process(originID model.Identifier, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *flow.ExecutionReceipt:
		err = e.onExecutionReceipt(originID, ev)
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}

	return nil
}

// onExecutionReceipt receives an execution receipt (exrcpt), verifies that and emits
// a result approval upon successful verification
func (e *Engine) onExecutionReceipt(originID model.Identifier, exrcpt *flow.ExecutionReceipt) error {
	// todo: add id of the ER once gets available
	e.log.Info().
		Hex("origin_id", originID[:]).
		Msg("execution receipt received")

	// todo: correctness check for execution receipts

	// validating identity of the originID
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		// todo: potential attack on authenticity
		return errors.Errorf("invalid origin id %s", originID[:])
	}

	// validating role of the originID
	// an execution receipt should be either coming from an execution node through the
	// Process method, or from the current verifier node itself through the Submit method
	if id.Role != flow.Role(flow.RoleExecution) && id.NodeID != e.me.NodeID() {
		// todo: potential attack on integrity
		return errors.Errorf("invalid role for generating an execution receipt, id: %s, role: %s", id.NodeID, id.Role)
	}

	// extracting list of consensus nodes' ids
	consIds, err := e.state.Final().
		Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		return errors.Wrap(err, "could not get identities")
	}

	// emitting a result approval to all consensus nodes
	resApprov := &verification.ResultApproval{}
	err = e.con.Submit(resApprov, consIds.NodeIDs()...)
	if err != nil {
		return errors.Wrap(err, "could not push result approval")
	}

	// todo: add a hex for hash of the result approval
	e.log.Info().
		Strs("target_ids", logging.HexSlice(consIds.NodeIDs())).
		Msg("result approval propagated")

	return nil
}
