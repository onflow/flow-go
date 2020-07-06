package provider

import (
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
)

type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	metrics  module.EngineMetrics
	me       module.Local
	state    protocol.State
	con      network.Conduit
	retrieve RetrieveFunc
	selector flow.IdentityFilter
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
		log:      log.With().Str("engine", "synchronization").Logger(),
		metrics:  metrics,
		me:       me,
		state:    state,
		retrieve: retrieve,
		selector: selector,
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
	switch ev := event.(type) {
	case *messages.ResourceRequest:
		e.before(metrics.MessageResourceRequest)
		defer e.after(metrics.MessageResourceRequest)
		return e.onResourceRequest(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) before(msg string) {
	e.metrics.MessageReceived(metrics.EngineSynchronization, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.metrics.MessageHandled(metrics.EngineSynchronization, msg)
}

func (e *Engine) onResourceRequest(originID flow.Identifier, req *messages.ResourceRequest) error {

	// TODO: track previous requests to protect against spam / repeated requests

	// TODO: add support for batch requests (multiple IDs in requests, multiple resources in response)

	// TODO: add delay to allow compounding of requests

	// then, we try to get the current identity of the requester and check it against the filter
	// for the handler to make sure the requester is authorized for this resource
	identities, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not retrieve identity for origin: %w", err)
	}
	if len(identities) == 0 {
		return fmt.Errorf("origin is not allowed to request resource")
	}

	// finally, execute the retrieve function to get the resource and send the response
	entity, err := e.retrieve(req.EntityID)
	if err != nil {
		return fmt.Errorf("could not retrieve entity: %w", err)
	}
	rep := &messages.ResourceResponse{
		Entity: entity,
		Nonce:  req.Nonce,
	}
	err = e.con.Submit(rep, originID)
	if err != nil {
		return fmt.Errorf("could not send resource response: %w", err)
	}

	return nil
}
