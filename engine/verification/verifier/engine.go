package verifier

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	unit       *engine.Unit         // used to control startup/shutdown
	log        zerolog.Logger       // used to log relevant actions
	colChannel network.Conduit      // used to get resources from collection nodes
	exeChannel network.Conduit      // used to get resources from execution nodes
	me         module.Local         // used to access local node information
	state      protocol.State       // used to access the  protocol state
	pool       verification.Mempool // used to maintain an in-memory pool for execution receipts
	wg         sync.WaitGroup       // used to keep track of the number of threads
	mu         sync.Mutex
}

// New creates and returns a new instance of a verifier engine
func New(loger zerolog.Logger, net module.Network, state protocol.State, me module.Local, pool verification.Mempool) (*Engine, error) {
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   loger,
		state: state,
		me:    me,
		pool:  pool,
		wg:    sync.WaitGroup{},
		mu:    sync.Mutex{},
	}

	var err error
	e.colChannel, err = net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on collection provider channel: %w", err)
	}

	e.exeChannel, err = net.Register(engine.ExecutionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution provider channel: %w", err)
	}

	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process receives and submits an event to the verifier engine for processing.
// It returns an error so the verifier engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.ExecutionReceipt:
		return e.onExecutionReceipt(originID, ev)
	case *flow.Collection:
		return e.onCollection(originID, ev)
	case *messages.CollectionResponse:
		return e.onCollection(originID, ev.)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onExecutionReceipt receives an execution receipt (exrcpt), verifies that and emits
// a result approval upon successful verification
func (e *Engine) onExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
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

	// storing the execution receipt in the store of the engine
	isUnique := e.pool.Put(receipt)
	if !isUnique {
		return errors.New("received duplicate execution receipt")
	}

	// starting the core verification in a separate thread
	e.wg.Add(1)
	go e.verify(originID, receipt)

	return nil
}

// verify is an internal component of the verifier engine.
// It receives an execution receipt and does the core verification process.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
// If the submission was successful it generates a result approval for the incoming ExecutionReceipt
// and submits that to the network
func (e *Engine) verify(originID flow.Identifier, receipt *flow.ExecutionReceipt) {
	// todo core verification happens here

	// extracting list of consensus nodes' ids
	consIDs, err := e.state.Final().
		Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		// todo this error needs more advance handling after MVP
		e.log.Error().
			Str("error: ", err.Error()).
			Msg("could not load the consensus nodes ids")
		e.wg.Done()
		return
	}

	// emitting a result approval to all consensus nodes
	resApprov := &flow.ResultApproval{
		Body: flow.ResultApprovalBody{
			ExecutionResultHash:  receipt.ExecutionResult.Fingerprint(),
			AttestationSignature: nil,
			ChunkIndexList:       nil,
			Proof:                nil,
			Spocks:               nil,
		},
		VerifierSignature: nil,
	}

	// broadcasting result approval to all consensus nodes
	e.broadcastResultApproval(resApprov, &consIDs)

}

// broadcastResultApproval receives a ResultApproval and list of consensus nodes IDs, and broadcasts
// the result approval to the consensus nodes
func (e *Engine) broadcastResultApproval(resApprov *flow.ResultApproval, consIDs *flow.IdentityList) {
	err := e.con.Submit(resApprov, consIDs.NodeIDs()...)
	if err != nil {
		// todo this error needs more advance handling after MVP
		e.log.Error().
			Str("error: ", err.Error()).
			Msg("could not push the result approval to the network")
		e.wg.Done()
		return
	}

	// todo: add a hex for hash of the result approval
	e.log.Info().
		Strs("target_ids", logging.HexSlice(consIDs.NodeIDs())).
		Msg("result approval propagated")
	e.wg.Done()
}
