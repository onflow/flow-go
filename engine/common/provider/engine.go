package provider

import (
	"fmt"
	"math/rand"
	"time"

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
	cfg      Config
	metrics  module.EngineMetrics
	me       module.Local
	state    protocol.State
	con      network.Conduit
	channel  uint8
	retrieve RetrieveFunc
	selector flow.IdentityFilter
	requests map[flow.Identifier]Request
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter, retrieve RetrieveFunc) (*Engine, error) {

	// initialize the default configuration
	cfg := Config{
		BatchThreshold: 128,
		BatchInterval:  time.Second,
	}

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
		cfg:      cfg,
		metrics:  metrics,
		me:       me,
		state:    state,
		channel:  channel,
		retrieve: retrieve,
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
	identities, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not retrieve identity for origin: %w", err)
	}
	if len(identities) == 0 {
		return fmt.Errorf("origin is not allowed to request resource")
	}

	// get the currently pending requested entities for the origin
	request, exists := e.requests[originID]
	if !exists {
		request = Request{
			OriginID:  originID,
			EntityIDs: nil,
			Timestamp: time.Now().UTC(),
		}
		e.requests[originID] = request
	}

	// add each entity ID to the requests
	// NOTE: we could punish here for duplicate requests
	// for _, entityID := range req.EntityIDs {
	// 	request.EntityIDs[entityID] = struct{}{}
	// }
	request.EntityIDs[req.EntityID] = struct{}{}

	// if the batch size is still too small, skip immediate processing
	if uint(len(request.EntityIDs)) < e.cfg.BatchThreshold {
		return nil
	}

	// otherwise, we sent this response now and zero out the entry
	err = e.processResponse(request)
	if err != nil {
		return fmt.Errorf("could not process response: %w", err)
	}

	return nil
}

func (e *Engine) processResponse(request Request) error {

	// remove the request from the map
	delete(e.requests, request.OriginID)

	// collect all entities for the given IDs
	entities := make([]flow.Entity, 0, len(request.EntityIDs))
	for entityID := range request.EntityIDs {
		entity, err := e.retrieve(entityID)
		if err != nil {
			e.log.Debug().Hex("entity", entityID[:]).Msg("unknown entity requested")
			continue
		}
		entities = append(entities, entity)
	}

	// if no entities are in the response, we bail now
	if len(entities) == 0 {
		e.log.Debug().Msg("no entities to send in resource response")
		return nil
	}

	// otherwise, send the response to the original requester
	rep := &messages.ResourceResponse{
		Nonce:    rand.Uint64(),
		Entities: entities,
	}
	err := e.con.Submit(rep, request.OriginID)
	if err != nil {
		return fmt.Errorf("could not send resource response: %w", err)
	}

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageResourceResponse)

	return nil
}
