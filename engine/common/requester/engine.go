package requester

import (
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog"
	"github.com/vmihailenco/msgpack"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/rand"
)

// HandleFunc is a function provided to the requester engine to handle an entity
// once it has been retrieved from a provider. The function should be non-blocking
// and errors should be handled internally within the function.
type HandleFunc func(originID flow.Identifier, entity flow.Entity)

// CreateFunc is a function that creates a `flow.Entity` with an underlying type
// so that we can properly decode entities transmitted over the network.
type CreateFunc func() flow.Entity

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
	channel  channels.Channel
	selector flow.IdentityFilter[flow.Identity]
	create   CreateFunc
	handle   HandleFunc

	// changing the following state variables must be guarded by unit.Lock()
	items                 map[flow.Identifier]*Item
	requests              map[uint64]*messages.EntityRequest
	forcedDispatchOngoing *atomic.Bool // to ensure only trigger dispatching logic once at any time
}

// New creates a new requester engine, operating on the provided network channel, and requesting entities from a node
// within the set obtained by applying the provided selector filter. The options allow customization of the parameters
// related to the batch and retry logic.
func New(log zerolog.Logger, metrics module.EngineMetrics, net network.EngineRegistry, me module.Local, state protocol.State,
	channel channels.Channel, selector flow.IdentityFilter[flow.Identity], create CreateFunc, options ...OptionFunc) (*Engine, error) {

	// initialize the default config
	cfg := Config{
		BatchThreshold:  32,
		BatchInterval:   time.Second,
		RetryInitial:    4 * time.Second,
		RetryFunction:   RetryGeometric(2),
		RetryMaximum:    2 * time.Minute,
		RetryAttempts:   math.MaxUint32,
		ValidateStaking: true,
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

	// make sure we don't send requests from self
	selector = filter.And(
		selector,
		filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
		filter.Not(filter.HasParticipationStatus(flow.EpochParticipationStatusEjected)),
	)

	// make sure we only send requests to nodes that are active in the current epoch and have positive weight
	if cfg.ValidateStaking {
		selector = filter.And(
			selector,
			filter.HasInitialWeight[flow.Identity](true),
			filter.HasParticipationStatus(flow.EpochParticipationStatusActive),
		)
	}

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:                  engine.NewUnit(),
		log:                   log.With().Str("engine", "requester").Logger(),
		cfg:                   cfg,
		metrics:               metrics,
		me:                    me,
		state:                 state,
		channel:               channel,
		selector:              selector,
		create:                create,
		handle:                nil,
		items:                 make(map[flow.Identifier]*Item),          // holds all pending items
		requests:              make(map[uint64]*messages.EntityRequest), // holds all sent requests
		forcedDispatchOngoing: atomic.NewBool(false),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channels.Channel(channel), e)
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
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), message)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given message from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel channels.Channel, originID flow.Identifier, message interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, message)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an message originating on the local node.
func (e *Engine) ProcessLocal(message interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), message)
	})
}

// Process processes the given message from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, message)
	})
}

// EntityByID adds an entity to the list of entities to be requested from the
// provider. It is idempotent, meaning that adding the same entity to the
// requester engine multiple times has no effect, unless the item has
// expired due to too many requests and has thus been deleted from the
// list. The provided selector will be applied to the set of valid providers on top
// of the global selector injected upon construction. It allows for finer-grained
// control over which subset of providers to request a given entity from, such as
// selection of a collection cluster. Use `filter.Any` if no additional selection
// is required. Checks integrity of response to make sure that we got entity that we were requesting.
func (e *Engine) EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
	e.addEntityRequest(entityID, selector, true)
}

// Query will request data through the request engine backing the interface.
// The additional selector will be applied to the subset
// of valid providers for the data and allows finer-grained control
// over which providers to request data from. Doesn't perform integrity check
// can be used to get entities without knowing their ID.
func (e *Engine) Query(key flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
	e.addEntityRequest(key, selector, false)
}

func (e *Engine) addEntityRequest(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity], checkIntegrity bool) {
	e.unit.Lock()
	defer e.unit.Unlock()

	// check if we already have an item for this entity
	_, duplicate := e.items[entityID]
	if duplicate {
		return
	}

	// otherwise, add a new item to the list
	item := &Item{
		EntityID:       entityID,
		NumAttempts:    0,
		LastRequested:  time.Time{},
		RetryAfter:     e.cfg.RetryInitial,
		ExtraSelector:  selector,
		checkIntegrity: checkIntegrity,
	}
	e.items[entityID] = item
}

// Force will force the requester engine to dispatch all currently
// valid batch requests.
func (e *Engine) Force() {
	// exit early in case a forced dispatch is currently ongoing
	if e.forcedDispatchOngoing.Load() {
		return
	}

	// using Launch to ensure the caller won't be blocked
	e.unit.Launch(func() {
		// using atomic bool to ensure there is at most one caller would trigger dispatching requests
		if e.forcedDispatchOngoing.CompareAndSwap(false, true) {
			count := uint(0)
			for {
				dispatched, err := e.dispatchRequest()
				if err != nil {
					e.log.Error().Err(err).Msg("could not dispatch requests")
					break
				}
				if !dispatched {
					e.log.Debug().Uint("requests", count).Msg("forced request dispatch")
					break
				}
				count++
			}
			e.forcedDispatchOngoing.Store(false)
		}
	})
}

func (e *Engine) poll() {
	ticker := time.NewTicker(e.cfg.BatchInterval)

PollLoop:
	for {
		select {
		case <-e.unit.Quit():
			break PollLoop

		case <-ticker.C:
			if e.forcedDispatchOngoing.Load() {
				return
			}

			dispatched, err := e.dispatchRequest()
			if err != nil {
				e.log.Error().Err(err).Msg("could not dispatch requests")
				continue PollLoop
			}
			if dispatched {
				e.log.Debug().Uint("requests", 1).Msg("regular request dispatch")
			}
		}
	}

	ticker.Stop()
}

// dispatchRequest dispatches a subset of requests (selection based on internal heuristic).
// While `dispatchRequest` sends a request (covering some but not necessarily all items),
// if and only if there is something to request. In other words it cannot happen that
// `dispatchRequest` sends no request, but there is something to be requested.
// The boolean return value indicates whether a request was dispatched at all.
func (e *Engine) dispatchRequest() (bool, error) {

	e.unit.Lock()
	defer e.unit.Unlock()

	e.log.Debug().Int("num_entities", len(e.items)).Msg("selecting entities")

	// get the current top-level set of valid providers
	providers, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return false, fmt.Errorf("could not get providers: %w", err)
	}

	// randomize order of items, so that they can be requested in different order each time
	rndItems := make([]flow.Identifier, 0, len(e.items))
	for k := range e.items {
		rndItems = append(rndItems, e.items[k].EntityID)
	}
	err = rand.Shuffle(uint(len(rndItems)), func(i, j uint) {
		rndItems[i], rndItems[j] = rndItems[j], rndItems[i]
	})
	if err != nil {
		return false, fmt.Errorf("shuffle failed: %w", err)
	}

	// go through each item and decide if it should be requested again
	now := time.Now().UTC()
	var providerID flow.Identifier
	var entityIDs []flow.Identifier
	for _, entityID := range rndItems {
		item := e.items[entityID]

		// if the item should not be requested yet, ignore
		cutoff := item.LastRequested.Add(item.RetryAfter)
		if cutoff.After(now) {
			continue
		}

		// if the item reached maximum amount of retries, drop
		if item.NumAttempts >= e.cfg.RetryAttempts {
			e.log.Debug().Str("entity_id", entityID.String()).Msg("dropping entity ID max amount of retries reached")
			delete(e.items, entityID)
			continue
		}

		// if the provider has already been chosen, check if this item
		// can be requested from the same provider; otherwise skip it
		// for now, so it will be part of the next batch request
		if providerID != flow.ZeroID {
			overlap := providers.Filter(filter.And(
				filter.HasNodeID[flow.Identity](providerID),
				item.ExtraSelector,
			))
			if len(overlap) == 0 {
				continue
			}
		}

		// if no provider has been chosen yet, choose from restricted set
		// NOTE: a single item can not permanently block requests going
		// out when no providers are available for it, because the iteration
		// order is random and will skip the item most of the times
		// when other items are available
		if providerID == flow.ZeroID {
			providers = providers.Filter(item.ExtraSelector)
			if len(providers) == 0 {
				return false, fmt.Errorf("no valid providers available")
			}
			id, err := providers.Sample(1)
			if err != nil {
				return false, fmt.Errorf("sampling failed: %w", err)
			}
			providerID = id[0].NodeID
		}

		// add item to list and set retry parameters
		// NOTE: we add the retry interval to the last requested timestamp,
		// rather than using the current timestamp, in order to conserve a
		// more even distribution of timestamps over time, which should lead
		// to a more even distribution of entities over batch requests
		entityIDs = append(entityIDs, entityID)
		item.NumAttempts++
		item.LastRequested = now
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
		return false, nil
	}

	nonce, err := rand.Uint64()
	if err != nil {
		return false, fmt.Errorf("nonce generation failed: %w", err)
	}

	// create a batch request, send it and store it for reference
	req := &messages.EntityRequest{
		Nonce:     nonce,
		EntityIDs: entityIDs,
	}

	requestStart := time.Now()

	if e.log.Debug().Enabled() {
		e.log.Debug().
			Hex("provider", logging.ID(providerID)).
			Uint64("nonce", req.Nonce).
			Int("num_selected", len(entityIDs)).
			Strs("entities", logging.IDs(entityIDs)).
			Msg("sending entity request")
	}

	err = e.con.Unicast(req, providerID)
	if err != nil {
		return true, fmt.Errorf("could not send request for entities %v: %w", logging.IDs(entityIDs), err)
	}
	e.requests[req.Nonce] = req

	if e.log.Debug().Enabled() {
		e.log.Debug().
			Hex("provider", logging.ID(providerID)).
			Uint64("nonce", req.Nonce).
			Strs("entities", logging.IDs(entityIDs)).
			TimeDiff("duration", time.Now(), requestStart).
			Msg("entity request sent")
	}

	// NOTE: we forget about requests after the expiry of the shortest retry time
	// from the entities in the list; this means that we purge requests aggressively.
	// However, most requests should be responded to on the first attempt and clearing
	// these up only removes the ability to instantly retry upon partial responses, so
	// it won't affect much.
	go func() {
		<-time.After(e.cfg.RetryInitial)

		e.unit.Lock()
		defer e.unit.Unlock()
		delete(e.requests, req.Nonce)
	}()

	e.metrics.MessageSent(e.channel.String(), metrics.MessageEntityRequest)
	e.log.Debug().
		Uint64("nonce", req.Nonce).
		Strs("entity_ids", flow.IdentifierList(req.EntityIDs).Strings()).
		Msg("entity request sent")

	return true, nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, message interface{}) error {

	e.metrics.MessageReceived(e.channel.String(), metrics.MessageEntityResponse)
	defer e.metrics.MessageHandled(e.channel.String(), metrics.MessageEntityResponse)

	switch msg := message.(type) {
	case *messages.EntityResponse:
		return e.onEntityResponse(originID, msg)
	default:
		return engine.NewInvalidInputErrorf("invalid message type (%T)", message)
	}
}

func (e *Engine) onEntityResponse(originID flow.Identifier, res *messages.EntityResponse) error {
	lg := e.log.With().Str("origin_id", originID.String()).Uint64("nonce", res.Nonce).Logger()

	lg.Debug().Strs("entity_ids", flow.IdentifierList(res.EntityIDs).Strings()).Msg("entity response received")

	if e.cfg.ValidateStaking {

		// check that the response comes from a valid provider
		providers, err := e.state.Final().Identities(filter.And(
			e.selector,
			filter.HasNodeID[flow.Identity](originID),
		))
		if err != nil {
			return fmt.Errorf("could not get providers: %w", err)
		}
		if len(providers) == 0 {
			return engine.NewInvalidInputErrorf("invalid provider origin (%x)", originID)
		}
	}

	if e.log.Debug().Enabled() {
		e.log.Debug().
			Hex("provider", logging.ID(originID)).
			Strs("entities", logging.IDs(res.EntityIDs)).
			Uint64("nonce", res.Nonce).
			Msg("onEntityResponse entries received")
	}

	e.unit.Lock()
	defer e.unit.Unlock()

	// build a list of needed entities; if not available, process anyway,
	// but in that case we can't re-queue missing items
	needed := make(map[flow.Identifier]struct{})
	req, exists := e.requests[res.Nonce]
	if exists {
		delete(e.requests, req.Nonce)
		for _, entityID := range req.EntityIDs {
			needed[entityID] = struct{}{}
		}
	}

	// ensure the response is correctly formed
	if len(res.Blobs) != len(res.EntityIDs) {
		return engine.NewInvalidInputErrorf("invalid response with %d blobs, %d IDs", len(res.Blobs), len(res.EntityIDs))
	}

	// process each entity in the response
	// NOTE: this requires engines to be somewhat idempotent, which is a good
	// thing, as it increases the robustness of their code
	for i := 0; i < len(res.Blobs); i++ {
		blob := res.Blobs[i]
		entityID := res.EntityIDs[i]

		// the entity might already have been returned in another response
		item, exists := e.items[entityID]
		if !exists {
			lg.Debug().Hex("entity_id", logging.ID(entityID)).Msg("entity not in items skipping")
			continue
		}

		// create the entity with underlying concrete type and decode blob
		entity := e.create()
		err := msgpack.Unmarshal(blob, &entity)
		if err != nil {
			return fmt.Errorf("could not decode entity: %w", err)
		}

		if item.checkIntegrity {
			actualEntityID := entity.ID()
			// validate that we got correct entity, exactly what we were expecting
			if entityID != actualEntityID {
				lg.Error().
					Hex("stated_entity_id", logging.ID(entityID)).
					Hex("provided_entity", logging.ID(actualEntityID)).
					Bool(logging.KeySuspicious, true).
					Msg("provided entity does not match stated ID")
				continue
			}
		}

		// remove from needed items and pending items
		delete(needed, entityID)
		delete(e.items, entityID)

		// process the entity
		go e.handle(originID, entity)
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
