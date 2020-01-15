package verifier

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	unit        *engine.Unit        // used to control startup/shutdown
	log         zerolog.Logger      // used to log relevant actions
	conduit     network.Conduit     // used to propagate result approvals
	me          module.Local        // used to access local node information
	state       protocol.State      // used to access the protocol state
	receipts    mempool.Receipts    // used to store execution receipts in memory
	blocks      mempool.Blocks      // used to store blocks in memory
	collections mempool.Collections // used to store collections in memory
}

// New creates and returns a new instance of a verifier engine.
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
) (*Engine, error) {

	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log,
		state: state,
		me:    me,
	}

	var err error

	e.conduit, err = net.Register(engine.ApprovalProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on approval provider channel: %w", err)
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
	switch event.(type) {
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// verify is an internal component of the verifier engine that handles
// the core verification process.
//
// It receives an execution receipt, requests and waits for dependent
// collections, then generates a result approval and submits it to
// consensus nodes.
func (e *Engine) verify(receipt *flow.ExecutionReceipt) error {

	result := receipt.ExecutionResult
	blockID := result.BlockID

	block, err := e.blocks.Get(blockID)
	if err != nil {
		return fmt.Errorf("could not get block (id=%s): %w", blockID, err)
	}

	for {

		// update which collections exist locally
		for collID, exists := range requiredCollections {
			if !exists && e.collections.Has(collID) {
				requiredCollections[collID] = true
			}
		}

		// if we're missing any collections, continue waiting
		for _, exists := range requiredCollections {
			if !exists {
				continue
			}
		}

		// TODO execute transactions and confirm execution result

		consensusNodes, err := e.state.Final().
			Identities(identity.HasRole(flow.RoleConsensus))
		if err != nil {
			// TODO this error needs more advance handling after MVP
			e.log.Error().
				Str("error: ", err.Error()).
				Msg("could not load the consensus nodes ids")
			return
		}

		approval := &flow.ResultApproval{
			ResultApprovalBody: flow.ResultApprovalBody{
				ExecutionResultID: receipt.ExecutionResult.ID(),
			},
		}

		// broadcast result approval to consensus nodes
		err = e.approvalsConduit.Submit(approval, consensusNodes.NodeIDs()...)
		if err != nil {
			// TODO this error needs more advance handling after MVP
			e.log.Error().
				Err(err).
				Msg("could not submit result approval to consensus nodes")
		}

		return
	}
}
