// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package propagation

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model"
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
	ready := make(chan struct{})
	go func() {
		close(ready)
	}()
	return ready
}

// Done returns a done channel that is closed once the engine has fully stopped.
// It closes the internal stop channel to signal all running go routines and
// then waits for them to finish using the wait group.
func (e *Engine) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		close(done)
	}()
	return done
}

// Submit allows us to submit local events to the propagation engine. The
// function logs errors internally, rather than returning it, which allows other
// engines to submit events in a non-blocking way by using a goroutine.
func (e *Engine) Submit(event interface{}) {

	// process the event with our own ID to indicate it's local
	err := e.Process(e.me.NodeID(), event)
	if err != nil {
		e.log.Error().Err(err).Msg("could not process local event")
	}
}

// Process processes the given propagation engine event. Events that are given
// to this function originate within the propagation engine on the node with the
// given origin ID.
func (e *Engine) Process(originID model.Identifier, event interface{}) error {
	var err error
	switch ev := event.(type) {
	case *collection.GuaranteedCollection:
		err = e.onGuaranteedCollection(originID, ev)
	default:
		err = errors.Errorf("invalid event type (%T)", event)
	}
	if err != nil {
		return errors.Wrap(err, "could not process event")
	}
	return nil
}

// onGuaranteedCollection is called when a new guaranteed collection is received
// from another node on the network.
func (e *Engine) onGuaranteedCollection(originID model.Identifier, coll *collection.GuaranteedCollection) error {

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
