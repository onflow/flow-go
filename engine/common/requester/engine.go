package requester

import (
	"fmt"
	"math"
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

// HandleFunc is a function provided to the requester engine to handle an entity
// once it has been retrieved from a provider. The function should ideally just
// error on some basic checks and be non-blocking for the further processing
// of the entity. In other words, errors in the processing logic of the handling
// engine should be part of the execution within that engine and happen in a
// separate goroutine.
type HandleFunc func(originID flow.Identifier, entity flow.Entity) error

// Engine is a generic requester engine, handling the requesting of entities
// on the flow network. It is the `request` part of the request-reply
// pattern provided by the pair of generic exchange engines.
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	cfg      Config
	metrics  module.EngineMetrics
	me       module.Local
	state    protocol.State
	con      network.Conduit
	channel  uint8
	selector flow.IdentityFilter
	handle   HandleFunc
	items    map[flow.Identifier]Item
	requests map[uint64]*messages.EntityRequest
}

// New creates a new requester engine, operating on the provided network channel, and requesting entities from a node
// within the set obtained by applying the provided selector filter. The options allow customization of the parameters
// related to the batch and retry logic.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter, options ...OptionFunc) (*Engine, error) {

	// initialize the default config
	cfg := Config{
		BatchThreshold: 32,
		BatchInterval:  time.Second,
		RetryInitial:   4 * time.Second,
		RetryFunction:  RetryGeometric(2),
		RetryMaximum:   2 * time.Minute,
		RetryAttempts:  math.MaxUint32,
	}

	// apply the custom option parameters
	for _, option := range options {
		option(&cfg)
	}

	// check validity of retry function
	interval := cfg.RetryFunction(time.Second)
	if interval < time.Second {
		return nil, fmt.Errorf("invalid retry function (interval must always increase)")
	}

	// check validity of maximum interval
	if cfg.RetryMaximum < cfg.RetryInitial {
		return nil, fmt.Errorf("invalid retry maximum (must not be smaller than initial interval)")
	}

	// make sure we don't send requests from self or unstaked nodes
	selector = filter.And(
		selector,
		filter.HasStake(true),
		filter.Not(filter.HasNodeID(me.NodeID())),
	)

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "requester").Logger(),
		cfg:      cfg,
		metrics:  metrics,
		me:       me,
		state:    state,
		channel:  channel,
		selector: selector,
		handle:   nil,
		items:    make(map[flow.Identifier]Item),           // holds all pending items
		requests: make(map[uint64]*messages.EntityRequest), // holds all sent requests
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// WithHandle sets the handle function of the requester, which is how it processes
// returned entities. The engine can not be started without setting the handle
// function. It is done in a separate call so that the requester can be injected
// into engines upon construction, and then provide a handle function to the
// requester from that engine itself.
func (e *Engine) WithHandle(handle HandleFunc) {
	e.handle = handle
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.handle == nil {
		panic("must initialize requester engine with handler")
	}
	e.unit.Launch(e.poll)
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
			e.log.Error().Err(err).Msg("requester engine could not process message")
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

// EntityByID adds an entity to the list of entities to be requested from the
// provider. It is idempotent, meaning that adding the same entity to the
// requester engine multiple times has no effect, unless the item has
// expired due to too many requests and has thus been deleted from the
// list.
func (e *Engine) EntityByID(entityID flow.Identifier) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	// check if we already have an item for this entity
	_, duplicate := e.items[entityID]
	if duplicate {
		return nil
	}

	// otherwise, add a new item to the list
	item := Item{
		EntityID:      entityID,
		NumAttempts:   0,
		LastRequested: time.Time{},
		RetryAfter:    e.cfg.RetryInitial,
	}
	e.items[entityID] = item

	return nil
}

func (e *Engine) poll() {
	ticker := time.NewTicker(e.cfg.BatchInterval)

PollLoop:
	for {
		select {
		case <-e.unit.Quit():
			break PollLoop

		case <-ticker.C:
			err := e.dispatchRequests()
			if err != nil {
				e.log.Error().Err(err).Msg("could not dispatch requests")
				continue PollLoop
			}
		}
	}

	ticker.Stop()
}

func (e *Engine) dispatchRequests() error {

	// go through each item and decide if it should be requested again
	now := time.Now().UTC()
	var entityIDs []flow.Identifier
	for entityID, item := range e.items {

		// if the item should not be requested yet, ignore
		if item.LastRequested.Add(item.RetryAfter).Before(now) {
			continue
		}

		// if the item reached maximum amount of retries, drop
		if item.NumAttempts >= e.cfg.RetryAttempts {
			delete(e.items, entityID)
			continue
		}

		// add item to list and set retry parameters
		// NOTE: we add the retry interval to the last requested timestamp,
		// rather than using the current timestamp, in order to conserve a
		// more even distribution of timestamps over time, which should lead
		// to a more even distribution of entities over batch requests
		entityIDs = append(entityIDs, entityID)
		item.NumAttempts++
		item.LastRequested = item.LastRequested.Add(item.RetryAfter)
		item.RetryAfter = e.cfg.RetryFunction(item.RetryAfter)

		// make sure the interval is within parameters
		if item.RetryAfter < e.cfg.RetryInitial {
			item.RetryAfter = e.cfg.RetryInitial
		}
		if item.RetryAfter > e.cfg.RetryMaximum {
			item.RetryAfter = e.cfg.RetryMaximum
		}

		// if we reached the maximum size for a batch, bail
		if uint(len(entityIDs)) >= e.cfg.BatchThreshold {
			break
		}
	}

	// if there are no items to request, return
	if len(entityIDs) == 0 {
		return nil
	}

	// pick a random target from the valid providers
	providers, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not get providers: %w", err)
	}
	if len(providers) == 0 {
		return fmt.Errorf("no valid providers available")
	}

	// create a batch request, send it and store it for reference
	targetID := providers.Sample(1)[0].NodeID
	req := &messages.EntityRequest{
		Nonce:     rand.Uint64(),
		EntityIDs: entityIDs,
	}
	err = e.con.Submit(req, targetID)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	e.requests[req.Nonce] = req

	// NOTE: we forget about requests after the expiry of the shortest retry time
	// from the entities in the list; this means that we purge requests aggressively.
	// However, most requests should be responded to on the first attempt and clearing
	// these up only removes the ability to instantly retry upon partial responses, so
	// it won't affect much.
	go func() {
		<-time.After(e.cfg.RetryInitial)
		delete(e.requests, req.Nonce)
	}()

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageEntityRequest)

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, message interface{}) error {

	e.metrics.MessageReceived(engine.ChannelName(e.channel), metrics.MessageEntityResponse)
	defer e.metrics.MessageHandled(engine.ChannelName(e.channel), metrics.MessageEntityResponse)

	e.unit.Lock()
	defer e.unit.Unlock()

	switch msg := message.(type) {
	case *messages.EntityResponse:
		return e.onEntityResponse(originID, msg)
	default:
		return engine.NewInvalidInputErrorf("invalid message type (%T)", message)
	}
}

func (e *Engine) onEntityResponse(originID flow.Identifier, res *messages.EntityResponse) error {

	// check that the response comes from a valid provider
	providers, err := e.state.Final().Identities(filter.And(
		e.selector,
		filter.HasNodeID(originID),
	))
	if err != nil {
		return fmt.Errorf("could not get providers: %w", err)
	}
	if len(providers) == 0 {
		return engine.NewInvalidInputErrorf("invalid provider origin (%x)", originID)
	}

	// build a list of needed entities; if not available, process anyway,
	// but in that case we can't re-queue missing items
	needed := make(map[flow.Identifier]struct{})
	req, exists := e.requests[res.Nonce]
	if exists {
		for _, entityID := range req.EntityIDs {
			needed[entityID] = struct{}{}
		}
	}

	// process each entity in the response
	// NOTE: this requires engines to be somewhat idempotent, which is a good
	// thing, as it increases the robustness of their code
	for _, entity := range res.Entities {

		// the entity might already have been returned in another response
		entityID := entity.ID()
		_, exists := e.items[entityID]
		if !exists {
			continue
		}

		// remove from needed items and pending items
		delete(needed, entityID)
		delete(e.items, entityID)

		// process the entity
		err := e.handle(originID, entity)
		if err != nil {
			return fmt.Errorf("could not handle entity (%x): %w", entityID, err)
		}
	}

	// requeue requested entities that have not been delivered in the response
	// NOTE: this logic allows a provider to send back an empty response to
	// indicate that none of the requested entities are available, thus allowing
	// the requester engine to immediately request them from another provider
	for entityID := range needed {

		// it's possible the item is unavailable, if it was already received
		// in another response
		item, exists := e.items[entityID]
		if !exists {
			// the entity could have been received in another request
			continue
		}

		// we set the timestamp to zero, so that the item will be included
		// in the next batch regardless of retry interval
		item.LastRequested = time.Time{}
	}

	return nil
}
