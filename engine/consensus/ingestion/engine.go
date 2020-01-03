// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
)

// Engine represents the ingestion engine, used to funnel collections from a
// cluster of collection nodes to the set of consensus nodes. It represents the
// link between collection nodes and consensus nodes and has a counterpart with
// the same engine ID in the collection node.
type Engine struct {
	unit  *engine.Unit    // used to manage concurrency & shutdown
	log   zerolog.Logger  // used to log relevant actions with context
	con   network.Conduit // used to talk to other nodes on the network
	prop  network.Engine  // used to process & propagate collections
	state protocol.State  // used to access the  protocol state
	me    module.Local    // used to access local node information
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, net module.Network, prop network.Engine, state protocol.State, me module.Local) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:  engine.NewUnit(),
		log:   log.With().Str("engine", "ingestion").Logger(),
		prop:  prop,
		state: state,
		me:    me,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
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
	switch ev := event.(type) {
	case *flow.CollectionGuarantee:
		return e.onCollectionGuarantee(originID, ev)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onCollectionGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
func (e *Engine) onCollectionGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_hash", guarantee.Hash()).
		Msg("guaranteed collection received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a guaranteed collection (usually collection nodes)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return errors.Wrap(err, "could not get origin node identity")
	}

	// check that the origin is a collection node; this check is fine even if it
	// excludes our own ID - in the case of local submission of collections, we
	// should use the propagation engine, which is for exchange of collections
	// between consensus nodes anyway; we do no processing or validation in this
	// engine beyond validating the origin
	if id.Role != flow.RoleCollection {
		return errors.Errorf("invalid origin node role (%s)", id.Role)
	}

	// submit the collection to the propagation engine - this is non-blocking
	// we could just validate it here and add it to the memory pool directly,
	// but then we would duplicate the validation logic
	e.prop.SubmitLocal(guarantee)

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_hash", guarantee.Hash()).
		Msg("guaranteed collection forwarded")

	return nil
}
