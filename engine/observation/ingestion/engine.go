// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	unit   *engine.Unit   // used to manage concurrency & shutdown
	log    zerolog.Logger // used to log relevant actions with context
	tracer trace.Tracer   // used to trace the data
	state  protocol.State // used to access the  protocol state
	me     module.Local   // used to access local node information

	// Conduits
	collectionConduit network.Conduit
	blockConduit      network.Conduit
	receiptConduit    network.Conduit
}

// New creates a new observation ingestion engine
func New(log zerolog.Logger, net module.Network, state protocol.State, tracer trace.Tracer, me module.Local) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	eng := &Engine{
		unit:   engine.NewUnit(),
		log:    log.With().Str("engine", "ingestion").Logger(),
		tracer: tracer,
		state:  state,
		me:     me,
	}

	blockConduit, err := net.Register(engine.BlockProvider, eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	collConduit, err := net.Register(engine.CollectionProvider, eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register collection provider engine")
	}

	receiptConduit, err := net.Register(engine.ExecutionReceiptProvider, eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register receipt provider engine")
	}

	eng.blockConduit = blockConduit
	eng.collectionConduit = collConduit
	eng.receiptConduit = receiptConduit

	return eng, nil
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
		err := e.process(originID, event)
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
	switch entity := event.(type) {
	case *flow.Block:
		return e.onBlock(originID, entity)
	case *flow.CollectionGuarantee:
		return e.onCollectionGuarantee(originID, entity)
	case *flow.Collection:
		return e.onCollection(originID, entity)
	case *flow.ExecutionReceipt:
		return e.onExecutionReceipt(originID, entity)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onBlock handles an incoming block.
func (e *Engine) onBlock(originID flow.Identifier, block *flow.Block) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("block_id", logging.Entity(block)).
		Uint64("block_view", block.View).
		Msg("received block")

	// TODO: Do something with block

	return nil
}

// onCollectionGuarantee is used to process collection guarantees received
// from nodes that are not consensus nodes (notably collection nodes).
func (e *Engine) onCollectionGuarantee(originID flow.Identifier, guarantee *flow.CollectionGuarantee) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("collection_id", logging.Entity(guarantee)).
		Msg("collection guarantee received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a collection guarantee (usually collection nodes)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return errors.Wrap(err, "could not get origin node identity")
	}

	// check that the origin is a collection node; this check is fine even if it
	// excludes our own ID - in the case of local submission of Collections, we
	// should use the propagation engine, which is for exchange of Collections
	// between consensus nodes anyway; we do no processing or validation in this
	// engine beyond validating the origin
	if id.Role != flow.RoleCollection {
		return errors.Errorf("invalid origin node role (%s)", id.Role)
	}
	//collID := guarantee.ID()
	//if !e.mempools.Collections.Has(collID) {
	//	// TODO rate limit these requests
	//	err := e.requestCollection(collID)
	//	if err != nil {
	//		log.Error().
	//			Err(err).
	//			Hex("collection_id", logging.ID(collID)).
	//			Msg("could not request collection")
	//	}
	//}

	return nil
}

// onCollection for handling data about a collection from a collection node
func (e *Engine) onCollection(originID flow.Identifier, guarantee *flow.Collection) error {
	// TODO
	return nil
}

// onExecutionReceipt handles an incoming execution receipts.
func (e *Engine) onExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	// TODO

	return nil
}

// requestCollection submits a request for the given collection to collection nodes.
func (e *Engine) requestCollection(collID flow.Identifier) error {

	collNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return fmt.Errorf("could not load collection node identities: %w", err)
	}

	req := &messages.CollectionRequest{
		ID: collID,
	}

	// TODO we should only submit to cluster which owns the collection
	err = e.collectionConduit.Submit(req, collNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit request for collection (id=%s): %w", collID, err)
	}

	return nil
}
