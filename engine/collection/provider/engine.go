// Package provider implements an engine for providing access to resources held
// by the collection node, including collections, collection guarantees, and
// transactions.
package provider

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the collection provider engine, which provides access to resources
// held by the collection node.
// TODO consolidate with common resource provider engine https://github.com/dapperlabs/flow-go/issues/4148
type Engine struct {
	unit         *engine.Unit
	log          zerolog.Logger
	engMetrics   module.EngineMetrics
	colMetrics   module.CollectionMetrics
	con          network.Conduit
	me           module.Local
	state        protocol.State
	pool         mempool.Transactions
	collections  storage.Collections
	transactions storage.Transactions
}

func New(log zerolog.Logger, net module.Network, state protocol.State, engMetrics module.EngineMetrics, colMetrics module.CollectionMetrics, me module.Local, pool mempool.Transactions, collections storage.Collections, transactions storage.Transactions) (*Engine, error) {
	e := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "provider").Logger(),
		engMetrics:   engMetrics,
		colMetrics:   colMetrics,
		me:           me,
		state:        state,
		pool:         pool,
		collections:  collections,
		transactions: transactions,
	}

	con, err := net.Register(engine.CollectionProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
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
			engine.LogError(e.log, err)
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

// process processes events for the provider engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.SubmitCollectionGuarantee:
		e.engMetrics.MessageReceived(metrics.EngineCollectionProvider, metrics.MessageSubmitGuarantee)
		defer e.engMetrics.MessageHandled(metrics.EngineCollectionProvider, metrics.MessageSubmitGuarantee)
		return e.onSubmitCollectionGuarantee(originID, ev)
	case *messages.CollectionRequest:
		e.engMetrics.MessageReceived(metrics.EngineCollectionProvider, metrics.MessageCollectionRequest)
		defer e.engMetrics.MessageHandled(metrics.EngineCollectionProvider, metrics.MessageCollectionRequest)
		return e.onCollectionRequest(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onSubmitCollectionGuarantee handles submitting the given collection guarantee
// to consensus nodes.
func (e *Engine) onSubmitCollectionGuarantee(originID flow.Identifier, req *messages.SubmitCollectionGuarantee) error {
	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote request to submit collection guarantee (from=%x)", originID)
	}

	return e.SubmitCollectionGuarantee(&req.Guarantee)
}

func (e *Engine) onCollectionRequest(originID flow.Identifier, req *messages.CollectionRequest) error {

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("collection_id", logging.ID(req.ID)).
		Logger()

	log.Debug().Msg("received collection request")

	coll, err := e.collections.ByID(req.ID)
	// we don't have the collection requested by other node
	if errors.Is(err, storage.ErrNotFound) {
		log.Warn().Err(err).Msg("requested collection not found")
		return nil
	}

	if err != nil {
		// running into some exception
		return fmt.Errorf("could not retrieve requested collection: %w", err)
	}

	res := &messages.CollectionResponse{
		Collection: *coll,
		Nonce:      req.Nonce,
	}
	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not respond to collection requester: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineCollectionProvider, metrics.MessageCollectionResponse)

	return nil
}

// SubmitCollectionGuarantee submits the collection guarantee to all
// consensus nodes.
func (e *Engine) SubmitCollectionGuarantee(guarantee *flow.CollectionGuarantee) error {

	consensusNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("could not get consensus nodes: %w", err)
	}

	err = e.con.Submit(guarantee, consensusNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit collection guarantee: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineCollectionProvider, metrics.MessageCollectionGuarantee)
	e.log.Debug().
		Hex("guarantee_id", logging.ID(guarantee.ID())).
		Hex("ref_block_id", logging.ID(guarantee.ReferenceBlockID)).
		Msg("submitting collection guarantee")

	return nil
}
