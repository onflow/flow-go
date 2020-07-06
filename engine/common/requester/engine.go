package requester

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
	"github.com/dapperlabs/flow-go/utils/logging"
)

type Engine struct {
	unit      *engine.Unit
	log       zerolog.Logger
	cfg       Config
	metrics   module.EngineMetrics
	me        module.Local
	state     protocol.State
	con       network.Conduit
	channel   uint8
	selector  flow.IdentityFilter
	queued    map[flow.Identifier]module.HandleFunc
	requested map[flow.Identifier]module.HandleFunc
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter) (*Engine, error) {

	// initialize the default config
	cfg := Config{
		BatchThreshold: 128,
		BatchInterval:  time.Second,
	}

	// make sure we don't send requests from self or unstaked nodes
	selector = filter.And(
		selector,
		filter.HasStake(true),
		filter.Not(filter.HasNodeID(me.NodeID())),
	)

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "requester").Logger(),
		cfg:       cfg,
		metrics:   metrics,
		me:        me,
		state:     state,
		channel:   channel,
		selector:  selector,
		queued:    make(map[flow.Identifier]module.HandleFunc),
		requested: make(map[flow.Identifier]module.HandleFunc),
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
func (e *Engine) Request(entityID flow.Identifier, handle module.HandleFunc) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	// TODO: keep track of in-flight requests to avoid duplicates

	// TODO: add automatic retrying and rotating valid recipients

	// skip entities that are already queued to be requested
	_, queued := e.queued[entityID]
	if queued {
		e.log.Debug().Hex("entity", entityID[:]).Msg("ignoring already queued entity")
		return nil
	}

	// skip duplicates without error for now
	_, requested := e.queued[entityID]
	if requested {
		e.log.Debug().Hex("entity", entityID[:]).Msg("ignoring already requested entity")
		return nil
	}

	// add the entity to the list of queued entities
	e.queued[entityID] = handle

	// if we have not reached the batch threshold, send now
	if uint(len(e.queued)) < e.cfg.BatchThreshold {
		return nil
	}

	// otherwise, create a new request and keep track
	err := e.executeRequest()
	if err != nil {
		return fmt.Errorf("could not execute request: %w", err)
	}

	return nil
}

func (e *Engine) executeRequest() error {

	// determine which identities are valid recipients of the request
	identities, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}
	if len(identities) == 0 {
		return fmt.Errorf("no valid targets for request available")
	}

	// put the list of pending entity IDs into the request
	entityIDs := make([]flow.Identifier, 0, len(e.queued))
	for entityID, handle := range e.queued {
		entityIDs = append(entityIDs, entityID)
		e.requested[entityID] = handle
		delete(e.queued, entityID)
	}
	req := &messages.ResourceRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: entityIDs,
	}

	// select a random recipient for the request and send
	target := identities[rand.Intn(len(identities))]
	err = e.con.Submit(&req, target.NodeID)
	if err != nil {
		return fmt.Errorf("could not send resource request: %w", err)
	}

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageResourceRequest)

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	switch ev := event.(type) {
	case *messages.ResourceResponse:
		e.metrics.MessageReceived(engine.ChannelName(e.channel), metrics.MessageResourceResponse)
		defer e.metrics.MessageHandled(engine.ChannelName(e.channel), metrics.MessageResourceResponse)
		return e.onResourceResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onResourceResponse(originID flow.Identifier, res *messages.ResourceResponse) error {

	// TODO: add reputation system to punish offenders of protocol conventions, slow responses

	// for each entity contained in the response, process it
	for _, entity := range res.Entities {
		err := e.processEntity(originID, entity)
		if err != nil {
			return fmt.Errorf("could not process entity (%x): %w", entity.ID(), err)
		}
	}

	return nil
}

func (e *Engine) processEntity(originID flow.Identifier, entity flow.Entity) error {

	// check if this entity is actually still pending
	handle, requested := e.requested[entity.ID()]
	if !requested {
		e.log.Debug().Hex("entity", logging.Entity(entity)).Msg("discarding non-requested entity")
		return nil
	}

	// remove the entity from the requested list
	delete(e.requested, entity.ID())

	// process the entity with the injected handle function
	err := handle(originID, entity)
	if err != nil {
		return fmt.Errorf("could not handle entity: %w", err)
	}

	return nil
}
