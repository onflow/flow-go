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

type HandleFunc func(originID flow.Identifier, entity flow.Entity) error

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
	requests map[uint64]*messages.ResourceRequest
}

// New creates a new consensus propagation engine.
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
		items:    make(map[flow.Identifier]Item),             // holds all pending items
		requests: make(map[uint64]*messages.ResourceRequest), // holds all sent requests
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// WithHandle will set the handler function to process entities.
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
		EntityID:  entityID,
		Attempts:  0,
		Timestamp: time.Time{},
		Interval:  e.cfg.RetryInitial,
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
		if item.Timestamp.Add(item.Interval).Before(now) {
			continue
		}

		// if the item reached maximum amount of retries, drop
		if item.Attempts >= e.cfg.RetryAttempts {
			delete(e.items, entityID)
			continue
		}

		// add item to list and set retry parameters
		entityIDs = append(entityIDs, entityID)
		item.Attempts++
		item.Timestamp = item.Timestamp.Add(item.Interval)
		item.Interval = e.cfg.RetryFunction(item.Interval)

		// make sure the interval is within parameters
		if item.Interval < e.cfg.RetryInitial {
			item.Interval = e.cfg.RetryInitial
		}
		if item.Interval > e.cfg.RetryMaximum {
			item.Interval = e.cfg.RetryMaximum
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
	targets, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return fmt.Errorf("could not retrieve targets: %w", err)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no request targets available")
	}
	targetID := targets.Sample(1)[0].NodeID

	// create a batch request, send it and store it for reference
	req := &messages.ResourceRequest{
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

	e.metrics.MessageSent(engine.ChannelName(e.channel), metrics.MessageResourceRequest)

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	e.metrics.MessageReceived(engine.ChannelName(e.channel), metrics.MessageResourceResponse)
	defer e.metrics.MessageHandled(engine.ChannelName(e.channel), metrics.MessageResourceResponse)

	e.unit.Lock()
	defer e.unit.Unlock()

	switch ev := event.(type) {
	case *messages.ResourceResponse:
		return e.onResourceResponse(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onResourceResponse(originID flow.Identifier, res *messages.ResourceResponse) error {

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
		item.Timestamp = time.Time{}
	}

	return nil
}
