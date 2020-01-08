// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package provider

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine represents the provider engine, used to spread block proposals across
// the flow system, to non-consensus nodes. It makes sense to use a separate
// engine to isolate the consensus algorithm from other processes, which allows
// to create a different underlying protocol for consensus nodes, which have a
// higher priority to receive block proposals, and other nodes
type Engine struct {
	unit  *engine.Unit    // used for concurrency & shutdown
	log   zerolog.Logger  // used to log relevant actions with context
	con   network.Conduit // used to talk to other nodes on the network
	state protocol.State  // used to access the  protocol state
	me    module.Local    // used to access local node information
}

// New creates a new block expulsion engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log.With().Str("engine", "expulsion").Logger(),
		state: state,
		me:    me,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ConsensusProvider, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the expulsion engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
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

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	var err error
	switch ev := event.(type) {
	// TODO: replace with real block proposal model
	case *coldstuff.BlockProposal:
		err = e.onBlockProposal(originID, ev)
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}

// onBlockProposal is used to process block proposals in regards to syncing
// non-consensus nodes on the network. It does not do any validation on the
// proposal itself, so should be called by the consensus engine only for valid
// block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, candidate *coldstuff.BlockProposal) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("block_hash", candidate.Block.Hash()).
		Msg("block proposal received")

	// currently, only accept blocks that come from our local consensus
	if originID != e.me.NodeID() {
		return errors.Errorf("non-local block proposal (nodeID: %x)", originID)
	}

	// get all non-consensus nodes in the system
	targetIDs, err := e.state.Final().Identities(identity.Not(identity.HasRole(flow.RoleConsensus)))
	if err != nil {
		return errors.Wrap(err, "could not retrieve target identities")
	}

	// submit the block proposal to the targets
	err = e.con.Submit(candidate, targetIDs.NodeIDs()...)
	if err != nil {
		return errors.Wrap(err, "could not send block proposal")
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("block_hash", candidate.Block.Hash()).
		Msg("block proposal propagated")

	return nil
}
