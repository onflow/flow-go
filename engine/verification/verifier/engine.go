package verifier

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine implements the verifier engine of the verification node,
// responsible for reception of a execution receipt, verifying that, and
// emitting its corresponding result approval to the entire system.
type Engine struct {
	unit    *engine.Unit    // used to control startup/shutdown
	log     zerolog.Logger  // used to log relevant actions
	conduit network.Conduit // used to propagate result approvals
	me      module.Local    // used to access local node information
	state   protocol.State  // used to access the protocol state
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
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *verification.CompleteExecutionResult:
		return e.verify(originID, resource)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// verify handles the core verification process. It accepts an execution
// result and all dependent resources, verifies the result, and emits a
// result approval if applicable.
//
// If any part of verification fails, an error is returned, indicating to the
// initiating engine that the verification must be re-tried.
// TODO assumes blocks with one chunk
func (e *Engine) verify(originID flow.Identifier, res *verification.CompleteExecutionResult) error {

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote origin for verify")
	}

	computedEndState, err := e.executeChunk(res)
	if err != nil {
		return fmt.Errorf("could not verify chunk: %w", err)
	}

	// TODO for now, we discard the computed end state and approve the ER
	_ = computedEndState

	consensusNodes, err := e.state.Final().
		Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not load consensus node IDs: %w", err)
	}

	approval := &flow.ResultApproval{
		ResultApprovalBody: flow.ResultApprovalBody{
			ExecutionResultID: res.Receipt.ExecutionResult.ID(),
		},
	}

	// broadcast result approval to consensus nodes
	err = e.conduit.Submit(approval, consensusNodes.NodeIDs()...)
	if err != nil {
		// TODO this error needs more advance handling after MVP
		return fmt.Errorf("could not submit result approval: %w", err)
	}

	return nil
}

// executeChunk executes the transactions for a single chunk and returns the
// resultant end state, or an error if execution failed.
func (e *Engine) executeChunk(res *verification.CompleteExecutionResult) (flow.StateCommitment, error) {
	rt := runtime.NewInterpreterRuntime()
	blockCtx := virtualmachine.NewBlockContext(rt, res.Block)

	getRegister := func(key string) ([]byte, error) {
		registers := res.ChunkStates[0].Registers

		val, ok := registers[key]
		if !ok {
			return nil, fmt.Errorf("missing register")
		}

		return val, nil
	}

	chunkView := state.NewView(getRegister)

	for _, tx := range res.Collections[0].Transactions {
		txView := chunkView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			chunkView.ApplyDelta(txView.Delta())
		}
	}

	// TODO compute and return state commitment

	return nil, nil
}
