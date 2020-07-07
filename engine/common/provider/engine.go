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

type RetrieveFunc func(flow.Identifier) (flow.Entity, error)

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

// New creates a new consensus propagation engine.
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
			e.log.Error().Err(err).Msg("synchronization could not process submitted event")
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
	e.unit.Lock()
	defer e.unit.Unlock()

	switch ev := event.(type) {
	case *messages.ResourceRequest:
		e.metrics.MessageReceived(engine.ChannelName(e.channel), metrics.MessageResourceRequest)
		defer e.metrics.MessageHandled(engine.ChannelName(e.channel), metrics.MessageResourceRequest)
		return e.onResourceRequest(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onResourceRequest(originID flow.Identifier, req *messages.ResourceRequest) error {

	// TODO: add reputation system to punish nodes for malicious behaviour (spam / repeated requests)

	// then, we try to get the current identity of the requester and check it against the filter
	// for the handler to make sure the requester is authorized for this resource
	identities, err := e.state.Final().Identities(filter.And(
		e.selector,
		filter.HasNodeID(originID)),
	)
	if err != nil {
		return fmt.Errorf("could not retrieve identity for origin: %w", err)
	}
	if len(identities) == 0 {
		return fmt.Errorf("origin is not allowed to request resource")
	}

	// try to retrieve each entity and skip missing ones
	entities := make([]flow.Entity, 0, len(req.EntityIDs))
	for _, entityID := range req.EntityIDs {
		entity, err := e.retrieve(entityID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("could not retrieve entity: %w", err)
		}
		entities = append(entities, entity)
	}

	// NOTE: we do _NOT_ avoid sending empty responses, as this will allow
	// the requester to know we don't have any of the requested entities, so
	// he can retry those immediately, rather than waiting

	// send back the response
	res := &messages.ResourceResponse{
		Nonce:    req.Nonce,
		Entities: entities,
	}
	err = e.con.Submit(res, originID)
	if err != nil {
		return fmt.Errorf("could not send response: %w", err)
	}

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageResourceResponse)

	return nil
}
