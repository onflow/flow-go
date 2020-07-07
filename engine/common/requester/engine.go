package requester

import (
	"fmt"
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
	selector flow.IdentityFilter
	handle   module.HandleFunc
	items    map[flow.Identifier]Item
	pending  uint64
	batches  map[uint64]Batch
	entities map[flow.Identifier]uint64
}

// New creates a new consensus propagation engine.
func New(log zerolog.Logger, metrics module.EngineMetrics, net module.Network, me module.Local, state protocol.State,
	channel uint8, selector flow.IdentityFilter, options ...OptionFunc) (*Engine, error) {

	// initialize the default config
	cfg := Config{
		BatchThreshold: 128,
		BatchInterval:  time.Second,
		RetryInitial:   4 * time.Second,
		RetryFunction:  RetryGeometric(2),
		RetryMaximum:   2 * time.Minute,
		RetryAttempts:  3,
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
		items:    make(map[flow.Identifier]Item),   // holds all pending items
		pending:  0,                                // identifies the batch currently being built
		batches:  make(map[uint64]Batch),           // holds a list of all active batches
		entities: make(map[flow.Identifier]uint64), // holds a list of all active entities
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
func (e *Engine) WithHandle(handle module.HandleFunc) {
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

// Request allows us to request an entity to be processed by the given callback.
func (e *Engine) Request(entityID flow.Identifier) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	return e.requestEntity(entityID)
}

func (e *Engine) requestEntity(entityID flow.Identifier) error {

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
		Nonce:     0,
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
	return nil
}

func (e *Engine) dispatchBatch(nonce uint64) error {

	// get the batch from the map
	batch, exists := e.batches[nonce]
	if !exists {
		return fmt.Errorf("unknown batch nonce (%d)", nonce)
	}

	// remove the batch from pending
	// NOTE: the batch will still be retried if it fails
	e.pending = 0

	// create the resource request and send to target
	req := &messages.ResourceRequest{
		Nonce:     batch.Nonce,
		EntityIDs: batch.EntityIDs,
	}
	err := e.con.Submit(&req, batch.TargetID)
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

	// check if there is a request pending with the given nonce
	batch, exists := e.batches[res.Nonce]
	if !exists {
		// NOTE: we can use alternative logic by entity ID if
		// we want to use outdated responses
		return nil
	}

	// create map of entities needed
	needed := make(map[flow.Identifier]struct{})
	for _, entityID := range batch.EntityIDs {
		needed[entityID] = struct{}{}
	}

	// go through all entities and check if it is still needed
	for _, entity := range res.Entities {
		entityID := entity.ID()
		_, requested := needed[entityID]
		if !requested {
			e.log.Warn().Hex("entity", entityID[:]).Msg("provider sent non-requested entity")
			continue
		}
		delete(needed, entityID)
		err := e.processEntity(originID, entity)
		if err != nil {
			return fmt.Errorf("could not process entity (%x): %w", entityID, err)
		}
	}

	// requeue all entities that have not been delivered yet
	for entityID := range needed {
		delete(e.entities, entityID)
		err := e.requestEntity(entityID)
		if err != nil {
			return fmt.Errorf("could not re-queue entity: %w", err)
		}
	}

	return nil
}

func (e *Engine) processEntity(originID flow.Identifier, entity flow.Entity) error {

	// remove the entity from the requested list
	delete(e.entities, entity.ID())

	// process the entity with the injected handle function
	err := e.handle(originID, entity)
	if err != nil {
		return fmt.Errorf("could not handle entity: %w", err)
	}

	return nil
}
