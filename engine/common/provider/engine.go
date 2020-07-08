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
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// RetrieveFunc is a function provided to the provider engine upon construction.
// It is used by the engine when receiving requests in order to retrieve the
// related entities. It is important that the retrieve function return a
// `storage.ErrNotFound` error if the entity does not exist locally; otherwise,
// the logic will error and not send responses when failing to retrieve entities.
type RetrieveFunc func(flow.Identifier) (flow.Entity, error)

// Engine is a generic provider engine, handling the fulfillment of entity
// requests on the flow network. It is the `reply` part of the request-reply
// pattern provided by the pair of generic exchange engines.
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	metrics  module.EngineMetrics
	me       module.Local
	state    protocol.State
	con      network.Conduit
	channel  uint8
	selector flow.IdentityFilter
	retrieve RetrieveFunc
}

// New creates a new provider engine, operating on the provided network channel, and accepting requests for entities
// from a node within the set obtained by applying the provided selector filter. It uses the injected retrieve function
// to manage the fullfilment of these requests.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter, retrieve RetrieveFunc) (*Engine, error) {

	// make sure we don't respond to requests sent by self or non-staked nodes
	selector = filter.And(
		selector,
		filter.HasStake(true),
		filter.Not(filter.HasNodeID(me.NodeID())),
	)

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "provider").Logger(),
		metrics:  metrics,
		me:       me,
		state:    state,
		channel:  channel,
		selector: selector,
		retrieve: retrieve,
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an message originating on the local node.
func (e *Engine) SubmitLocal(message interface{}) {
	e.Submit(e.me.NodeID(), message)
}

// Submit submits the given message from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, message interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, message)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an message originating on the local node.
func (e *Engine) ProcessLocal(message interface{}) error {
	return e.Process(e.me.NodeID(), message)
}

// Process processes the given message from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, message interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, message)
	})
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, message interface{}) error {

	e.metrics.MessageReceived(engine.ChannelName(e.channel), metrics.MessageEntityRequest)
	defer e.metrics.MessageHandled(engine.ChannelName(e.channel), metrics.MessageEntityRequest)

	e.unit.Lock()
	defer e.unit.Unlock()

	switch msg := message.(type) {
	case *messages.EntityRequest:
		return e.onEntityRequest(originID, msg)
	default:
		return engine.NewInvalidInputErrorf("invalid message type (%T)", message)
	}
}

func (e *Engine) onEntityRequest(originID flow.Identifier, req *messages.EntityRequest) error {

	// TODO: add reputation system to punish nodes for malicious behaviour (spam / repeated requests)

	// then, we try to get the current identity of the requester and check it against the filter
	// for the handler to make sure the requester is authorized for this resource
	requesters, err := e.state.Final().Identities(filter.And(
		e.selector,
		filter.HasNodeID(originID)),
	)
	if err != nil {
		return fmt.Errorf("could not get requesters: %w", err)
	}
	if len(requesters) == 0 {
		return engine.NewInvalidInputErrorf("invalid requester origin (%x)", originID)
	}

	// try to retrieve each entity and skip missing ones
	entities := make([]flow.Entity, 0, len(req.EntityIDs))
	for _, entityID := range req.EntityIDs {
		entity, err := e.retrieve(entityID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("could not retrieve entity (%x): %w", entityID, err)
		}
		entities = append(entities, entity)
	}

	// NOTE: we do _NOT_ avoid sending empty responses, as this will allow
	// the requester to know we don't have any of the requested entities, which
	// allows him to retry them immediately, rather than waiting for the expiry
	// of the retry interval

	// send back the response
	res := &messages.EntityResponse{
		Nonce:    req.Nonce,
		Entities: entities,
	}
	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send response: %w", err)
	}

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageEntityResponse)

	return nil
}
