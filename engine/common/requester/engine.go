package requester

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/vmihailenco/msgpack/v4"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/onflow/flow-go/utils/rand"
)

// DefaultEntityRequestCacheSize is the default max message queue size for the provider engine.
// This equates to ~5GB of memory usage with a full queue (10M*500)
const DefaultEntityRequestCacheSize = 500

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
	*component.ComponentManager
	mu             sync.Mutex
	log            zerolog.Logger
	cfg            Config
	metrics        module.EngineMetrics
	me             module.Local
	state          protocol.State
	con            network.Conduit
	channel        channels.Channel
	requestHandler *engine.MessageHandler
	requestQueue   engine.MessageStore
	selector       flow.IdentityFilter[flow.Identity]
	create         CreateFunc
	handle         HandleFunc

	// changing the following state variables must be guarded by mu.Lock()
	items                 map[flow.Identifier]*Item
	requests              map[uint64]*messages.EntityRequest
	forcedDispatchOngoing *atomic.Bool // to ensure only trigger dispatching logic once at any time
}

var _ component.Component = (*Engine)(nil)
var _ network.MessageProcessor = (*Engine)(nil)

// New creates a new requester engine, operating on the provided network channel, and requesting entities from a node
// within the set obtained by applying the provided selector filter. The options allow customization of the parameters
// related to the batch and retry logic.
// No error returns are expected during normal operations.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.EngineRegistry,
	me module.Local,
	state protocol.State,
	requestQueue engine.MessageStore,
	channel channels.Channel,
	selector flow.IdentityFilter[flow.Identity],
	create CreateFunc,
	options ...OptionFunc,
) (*Engine, error) {

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

	// make sure we don't send requests from self
	selector = filter.And(
		selector,
		filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
		filter.Not(filter.HasParticipationStatus(flow.EpochParticipationStatusEjected)),
	)

	// make sure we only send requests to nodes that are active in the current epoch and have positive weight
	selector = filter.And(
		selector,
		filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
		filter.Not(filter.HasParticipationStatus(flow.EpochParticipationStatusEjected)),
		filter.HasInitialWeight[flow.Identity](true),
	)

	handler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			// Match is called on every new message coming to this engine.
			// Provider engine only expects *flow.EntityResponse.
			// Other message types are discarded by Match.
			Match: func(message *engine.Message) bool {
				_, ok := message.Payload.(*flow.EntityResponse)
				return ok
			},
			Store: requestQueue,
		})

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:                   log.With().Str("engine", "requester").Logger(),
		cfg:                   cfg,
		metrics:               metrics,
		me:                    me,
		state:                 state,
		requestHandler:        handler,
		requestQueue:          requestQueue,
		channel:               channel,
		selector:              selector,
		create:                create,
		handle:                nil,
		items:                 make(map[flow.Identifier]*Item),          // holds all pending items
		requests:              make(map[uint64]*messages.EntityRequest), // holds all sent requests
		forcedDispatchOngoing: atomic.NewBool(false),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.poll).
		AddWorker(e.processQueuedRequestsShovellerWorker).
		Build()

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

// Process processes the given message from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.ShutdownSignal():
		e.log.Warn().
			Hex("origin_id", logging.ID(originID)).
			Msgf("received message after shutdown")
		return nil
	default:
	}

	e.metrics.MessageReceived(e.channel.String(), metrics.MessageEntityResponse)
	err := e.requestHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().
				Hex("origin_id", logging.ID(originID)).
				Str("channel", channel.String()).
				Str("event", fmt.Sprintf("%+v", event)).
				Bool(logging.KeySuspicious, true).
				Msg("received unsupported message type")
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine event: %w", err)
	}
	return nil
}

// processQueuedRequestsShovellerWorker runs as a dedicated worker for [component.ComponentManager].
// It tracks when there is available work and performs dispatch of incoming messages.
func (e *Engine) processQueuedRequestsShovellerWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	e.log.Debug().Msg("process entity request shoveller worker started")

	for {
		select {
		case <-e.requestHandler.GetNotifier():
			// there is at least a single request in the queue, so we try to process it.
			e.processAvailableMessages(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// processAvailableMessages is called when there are messages in the queue that are ready to be processed.
// All unexpected errors are reported to the SignalerContext.
func (e *Engine) processAvailableMessages(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, ok := e.requestQueue.Get()
		if !ok {
			// no more requests, return
			return
		}

		res, ok := msg.Payload.(*flow.EntityResponse)
		if !ok {
			// should never happen, as we only put EntityRequest in the queue,
			// if it does happen, it means there is a bug in the queue implementation.
			ctx.Throw(fmt.Errorf("invalid message type in entity request queue: %T", msg.Payload))
		}

		err := e.onEntityResponse(msg.OriginID, res)
		if err != nil {
			if engine.IsInvalidInputError(err) {
				e.log.Err(err).
					Str("origin_id", msg.OriginID.String()).
					Uint64("nonce", res.Nonce).
					Bool(logging.KeySuspicious, true).
					Msg("invalid response detected")
				continue
			}
			ctx.Throw(err)
		}
	}
}

// EntityByID will enqueue the given entity for request by its ID (content hash).
// The selector will be applied to the subset of valid providers configured globally for the Requester instance.
// This allows finer-grained control over which providers to request from on a per-entity basis.
// Use `filter.Any` if no additional restrictions are required.
// Received entities will be verified for integrity using their ID function.
func (e *Engine) EntityByID(entityID flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
	e.addEntityRequest(entityID, selector, true)
}

// EntityBySecondaryKey will enqueue the given entity for request by some secondary identifier (NOT its content hash).
// The selector will be applied to the subset of valid providers configured globally for the Requester instance.
// This allows finer-grained control over which providers to request from on a per-entity basis.
// Use `filter.Any` if no additional restrictions are required.
// Received entities WILL NOT be verified for integrity using their ID function.
func (e *Engine) EntityBySecondaryKey(key flow.Identifier, selector flow.IdentityFilter[flow.Identity]) {
	e.addEntityRequest(key, selector, false)
}

// addEntityRequest adds request in in-memory storage of pending items to be requested.
// Concurrency safe.
func (e *Engine) addEntityRequest(queryKey flow.Identifier, selector flow.IdentityFilter[flow.Identity], queryKeyIsContentHash bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// check if we already have an item for this entity
	_, duplicate := e.items[queryKey]
	if duplicate {
		return
	}

	// otherwise, add a new item to the list
	item := &Item{
		EntityID:           queryKey,
		NumAttempts:        0,
		LastRequested:      time.Time{},
		RetryAfter:         e.cfg.RetryInitial,
		ExtraSelector:      selector,
		queryByContentHash: queryKeyIsContentHash,
	}
	e.items[queryKey] = item
}

// Force will force the requester engine to dispatch all currently
// valid batch requests.
func (e *Engine) Force() {
	// exit early in case a forced dispatch is currently ongoing
	if e.forcedDispatchOngoing.Load() {
		return
	}

	// using Launch to ensure the caller won't be blocked
	go func() {
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
	}()
}

// poll runs as a dedicated worker for [component.ComponentManager]. It performs dispatch of pending requests using a timer.
func (e *Engine) poll(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	if e.handle == nil {
		ctx.Throw(fmt.Errorf("must initialize requester engine with handler"))
	}

	ready()

	ticker := time.NewTicker(e.cfg.BatchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if e.forcedDispatchOngoing.Load() {
				continue
			}

			dispatched, err := e.dispatchRequest()
			if err != nil {
				ctx.Throw(err)
			}
			if dispatched {
				e.log.Debug().Uint("requests", 1).Msg("regular request dispatch")
			}
		}
	}
}

// dispatchRequest dispatches a subset of requests (selection based on internal heuristic).
// While `dispatchRequest` sends a request (covering some but not necessarily all items),
// if and only if there is something to request. In other words it cannot happen that
// `dispatchRequest` sends no request, but there is something to be requested.
// The boolean return value indicates whether a request was dispatched at all.
// No error returns are expected during normal operations.
func (e *Engine) dispatchRequest() (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.log.Debug().Int("num_entities", len(e.items)).Msg("selecting entities")

	// get the current top-level set of valid providers
	providers, err := e.state.Final().Identities(e.selector)
	if err != nil {
		return false, fmt.Errorf("could not get providers: %w", err)
	}

	// go through each item and decide if it should be requested again
	now := time.Now().UTC()
	var providerID flow.Identifier
	var entityIDs []flow.Identifier
	for entityID, item := range e.items {

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

		// If no provider has been chosen yet, select one that:
		// - is part of the previously determined `providers` set (staked, non-ejected nodes)
		// - and matches the item's specific requirements (as per ExtraSelector)
		// NOTE: a single item can not permanently block requests going out when no providers are available for it,
		// because the iteration order is random. The `ExtraSelector` of the item that is iterated over first (at
		// random) will determine the selected provider.
		if providerID == flow.ZeroID {
			filteredProviders := providers.Filter(item.ExtraSelector)
			if len(filteredProviders) == 0 {
				e.log.Error().Msgf("could not dispatch requests: no valid providers available for item %s, total providers: %v", entityID.String(), len(providers))
				return false, nil
			}
			// Randomly select a provider from the eligible set. We will ask this data provider for all entities, whose `ExtraSelector`
			// matches this provider. Thereby, we maximize the batch size, requesting as many entities as possible via a single message.
			id, err := filteredProviders.Sample(1)
			if err != nil {
				return false, fmt.Errorf("sampling failed: %w", err)
			}
			providerID = id[0].NodeID
			providers = filteredProviders
		}

		// Add item to list and update the retry parameters.
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
		e.log.Error().Err(err).Msgf("could not dispatch requests: could not send request for entities %v", logging.IDs(entityIDs))
		return false, nil
	}
	e.requests[req.Nonce] = req

	// NOTE: we forget about requests after the expiry of the shortest retry time
	// from the entities in the list; this means that we purge requests aggressively.
	// However, most requests should be responded to on the first attempt and clearing
	// these up only removes the ability to instantly retry upon partial responses, so
	// it won't affect much.
	go func() {
		<-time.After(e.cfg.RetryInitial)

		e.mu.Lock()
		delete(e.requests, req.Nonce)
		e.mu.Unlock()
	}()

	if e.log.Debug().Enabled() {
		e.log.Debug().
			Hex("provider", logging.ID(providerID)).
			Uint64("nonce", req.Nonce).
			Strs("entities", logging.IDs(entityIDs)).
			TimeDiff("duration", time.Now(), requestStart).
			Msg("entity request sent")
	}
	e.metrics.MessageSent(e.channel.String(), metrics.MessageEntityRequest)

	return true, nil
}

// onEntityResponse handles response for request that was originally made by the engine.
// For each successful response this function spawns a dedicated go routine to perform handling of the parsed response.
// Considering the fact we process only responses that we have previously requested it's impossible to force this function to
// spawn arbitrary number of goroutines.
// Expected errors during normal operations:
//   - [engine.InvalidInputError] if the provided response is malformed
func (e *Engine) onEntityResponse(originID flow.Identifier, res *flow.EntityResponse) error {
	defer e.metrics.MessageHandled(e.channel.String(), metrics.MessageEntityResponse)
	lg := e.log.With().Str("origin_id", originID.String()).Uint64("nonce", res.Nonce).Logger()

	lg.Debug().Strs("entity_ids", flow.IdentifierList(res.EntityIDs).Strings()).Msg("entity response received")

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

	if e.log.Debug().Enabled() {
		e.log.Debug().
			Hex("provider", logging.ID(originID)).
			Strs("entities", logging.IDs(res.EntityIDs)).
			Uint64("nonce", res.Nonce).
			Msg("onEntityResponse entries received")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

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
			return engine.NewInvalidInputErrorf("could not decode entity: %s", err.Error())
		}

		if item.queryByContentHash {
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
		// TODO: We should update users of requester engine to uniformly pass in a non-blocking `handle` function
		// (Currently all users except the execution ingestion engine have non-blocking handlers: https://github.com/onflow/flow-go/blob/be489481bff28f42bc887fe26fe19476585ab6aa/engine/execution/ingestion/machine.go#L99)
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
