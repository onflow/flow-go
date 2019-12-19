// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/collection"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the propagation engine, which makes sure that new collections are
// propagated to the other consensus nodes on the network.
type Engine struct {
	unit  *engine.Unit          // used to control startup/shutdown
	log   zerolog.Logger        // used to log relevant actions with context
	con   network.Conduit       // used to talk to other nodes on the network
	state protocol.State        // used to access the  protocol state
	me    module.Local          // used to access local node information
	pool  module.CollectionPool // holds guaranteed collections in memory
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, pool module.CollectionPool) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log.With().Str("engine", "propagation").Logger(),
		state: state,
		me:    me,
		pool:  pool,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ConsensusPropagation, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// when we have received the first guaranteed collection for the memory pool.
// We thus start polling and wait until the memory pool has a size of at least
// one.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It closes the internal stop channel to signal all running go routines and
// then waits for them to finish using the wait group.
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
	switch ev := event.(type) {
	case *collection.GuaranteedCollection:
		return e.onGuaranteedCollection(originID, ev)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onGuaranteedCollection is called when a new guaranteed collection is received
// from another node on the network.
func (e *Engine) onGuaranteedCollection(originID flow.Identifier, coll *collection.GuaranteedCollection) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_hash", coll.Hash()).
		Msg("fingerprint message received")

	// process the guaranteed collection to make sure it's valid and new
	err := e.processGuaranteedCollection(coll)
	if err != nil {
		return errors.Wrap(err, "could not process collection")
	}

	// propagate the guaranteed collection to other relevant nodes
	err = e.propagateGuaranteedCollection(coll)
	if err != nil {
		return errors.Wrap(err, "could not broadcast collection")
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_hash", coll.Hash()).
		Msg("guaranteed collection processed")

	return nil
}

// processGuaranteedCollection will process a guaranteed collection within the
// context of our local protocol state and memory pool.
func (e *Engine) processGuaranteedCollection(coll *collection.GuaranteedCollection) error {

	// TODO: validate the guaranteed collection signature

	// add the guaranteed collection to our memory pool (also checks existence)
	err := e.pool.Add(coll)
	if err != nil {
		return errors.Wrap(err, "could not add collection to mempool")
	}

	return nil
}

// propagateGuaranteedCollection will submit the guaranteed collection to the
// network layer with all other consensus nodes as desired recipients.
func (e *Engine) propagateGuaranteedCollection(coll *collection.GuaranteedCollection) error {

	// select all the collection nodes on the network as our targets
	ids, err := e.state.Final().Identities(
		identity.HasRole(flow.RoleConsensus),
		identity.Not(identity.HasNodeID(e.me.NodeID())),
	)
	if err != nil {
		return errors.Wrap(err, "could not get identities")
	}

	// send the guaranteed collection to all consensus identities
	targetIDs := ids.NodeIDs()
	err = e.con.Submit(coll, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not push guaranteed collection")
	}

	e.log.Info().
		Strs("target_ids", logging.HexSlice(targetIDs)).
		Hex("collection_hash", coll.Hash()).
		Msg("guaranteed collection propagated")

	return nil
}
