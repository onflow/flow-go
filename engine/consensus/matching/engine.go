// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the propagation engine, which makes sure that new collections are
// propagated to the other consensus nodes on the network.
type Engine struct {
	unit      *engine.Unit      // used to control startup/shutdown
	log       zerolog.Logger    // used to log relevant actions with context
	con       network.Conduit   // used to talk to other nodes on the network
	state     protocol.State    // used to access the  protocol state
	me        module.Local      // used to access local node information
	receipts  mempool.Receipts  // holds collection guarantees in memory
	approvals mempool.Approvals // holds result approvals in memory
	seals     mempool.Seals     // holds block seals in memory
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, receipts mempool.Receipts, approvals mempool.Approvals, seals mempool.Seals) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "matching").Logger(),
		state:     state,
		me:        me,
		receipts:  receipts,
		approvals: approvals,
		seals:     seals,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ConsensusMatching, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the propagation engine, it closes the channel when all submit goroutines
// have ended.
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

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *flow.ExecutionReceipt:
		return e.onReceipt(originID, entity)
	case *flow.ResultApproval:
		return e.onApproval(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onReceipt processes a new execution receipt.
func (e *Engine) onReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.ID(receipt)).
		Msg("execution receipt received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin identity: %w", err)
	}

	// check that the origin is an execution node
	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid origin node role (%s)", id.Role)
	}

	// store in the memory pool
	err = e.receipts.Add(receipt)
	if err != nil {
		return fmt.Errorf("could not store receipt: %w", err)
	}

	// TODO: check if we have already enough matching approvals

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.ID(receipt)).
		Msg("execution receipt processed")

	return nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.ID(approval)).
		Msg("result approval received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a result approval (usually verification node)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin identity: %w", err)
	}

	// check that the origin is a verification node
	if id.Role != flow.RoleVerification {
		return fmt.Errorf("invalid origin node role (%s)", id.Role)
	}

	// store in the memory pool
	err = e.approvals.Add(approval)
	if err != nil {
		return fmt.Errorf("could not store approval: %w", err)
	}

	// TODO: check if we have already enough matching approvals

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.ID(approval)).
		Msg("execution receipt forwarded")

	return nil
}
