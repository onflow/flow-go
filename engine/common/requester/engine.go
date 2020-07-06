package requester

import (
	"fmt"
	"math/rand"

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
	selector flow.IdentityFilter
	requests map[flow.Identifier]Request
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter) (*Engine, error) {

	// make sure we don't send requests from self or unstaked nodes
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
		selector: selector,
		requests: make(map[flow.Identifier]Request),
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

// Request allows us to request an entity to be processed by the given callback.
func (e *Engine) Request(entityID flow.Identifier, process module.ProcessFunc) error {

	// TODO: keep track of in-flight requests to avoid duplicates

	// TODO: add delay so we can compound requests

	// TODO: add automatic retrying and rotating valid recipients

	// TODO: protect against race-conditions upon concurrent request/reply

	// determine which identities are valid recipients of the request
	identities, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}
	if len(identities) == 0 {
		return fmt.Errorf("no valid targets for request available")
	}

	// select a random recipient for the request
	target := identities[rand.Intn(len(identities))]
	req := &messages.ResourceRequest{
		EntityID: entityID,
		Nonce:    rand.Uint64(),
	}
	err = e.con.Submit(&req, target.NodeID)
	if err != nil {
		return fmt.Errorf("could not send resource request: %w", err)
	}

	// store the request for later reference
	request := Request{
		TargetID: target.NodeID,
		EntityID: entityID,
		Process:  process,
	}
	e.requests[req.EntityID] = request

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *messages.ResourceResponse:
		e.before(metrics.MessageResourceRequest)
		defer e.after(metrics.MessageResourceRequest)
		return e.onResourceResponse(originID, ev)
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

func (e *Engine) onResourceResponse(originID flow.Identifier, res *messages.ResourceResponse) error {

	// TODO: add support for batch requests & responses

	// TODO: add reputation system to punish offenders of protocol conventions, slow responses

	// check if we got a pending request for the given resource to the given target
	for _, entity := range res.Entities {
		request, ok := e.requests[entity.ID()]
		if !ok {
			return fmt.Errorf("response for unknown request nonce (%d)", res.Nonce)
		}
		if originID != request.TargetID {
			return fmt.Errorf("response origin mismatch with target (%x != %x)", originID, request.TargetID)
		}
		if entity.ID() != request.EntityID {
			return fmt.Errorf("response entity mismatch with requested (%x != %x)", entity.ID(), request.EntityID)
		}

		// forward the entity to the process function
		request.Process(originID, entity)
	}

	return nil
}
