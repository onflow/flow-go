package provider

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/provider/internal"
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
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultRequestProviderWorkers is the default number of workers used to process entity requests.
	DefaultRequestProviderWorkers = uint(5)

	// DefaultEntityRequestCacheSize is the default max message queue size for the provider engine.
	// This equates to ~5GB of memory usage with a full queue (10M*500)
	DefaultEntityRequestCacheSize = 500
)

// RetrieveFunc is a function provided to the provider engine upon construction.
// It is used by the engine when receiving requests in order to retrieve the
// related entities. It is important that the retrieve function return a
// `storage.ErrNotFound` error if the entity does not exist locally; otherwise,
// the logic will error and not send responses when failing to retrieve entities.
type RetrieveFunc func(flow.Identifier) (flow.Entity, error)

// Engine is a generic provider engine, handling the fulfillment of entity
// requests on the flow network. It is the `reply` part of the request-reply
// pattern provided by the pair of generic exchange engines.
type Engine struct {
	component.Component
	cm             *component.ComponentManager
	log            zerolog.Logger
	metrics        module.EngineMetrics
	state          protocol.State
	con            network.Conduit
	channel        channels.Channel
	requestHandler *engine.MessageHandler
	requestQueue   engine.MessageStore
	selector       flow.IdentityFilter[flow.Identity]
	retrieve       RetrieveFunc
	// buffered channel for EntityRequest workers to pick and process.
	requestChannel chan *internal.EntityRequest
}

var _ network.MessageProcessor = (*Engine)(nil)

// New creates a new provider engine, operating on the provided network channel, and accepting requests for entities
// from a node within the set obtained by applying the provided selector filter. It uses the injected retrieve function
// to manage the fullfilment of these requests.
func New(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.EngineRegistry,
	me module.Local,
	state protocol.State,
	requestQueue engine.MessageStore,
	requestWorkers uint,
	channel channels.Channel,
	selector flow.IdentityFilter[flow.Identity],
	retrieve RetrieveFunc) (*Engine, error) {

	// make sure we don't respond to request sent by self or unauthorized nodes
	selector = filter.And(
		selector,
		filter.Not(filter.HasNodeID[flow.Identity](me.NodeID())),
	)

	handler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			// Match is called on every new message coming to this engine.
			// Provider engine only expects EntityRequest.
			// Other message types are discarded by Match.
			Match: func(message *engine.Message) bool {
				_, ok := message.Payload.(*messages.EntityRequest)
				return ok
			},
			// Map is called on messages that are Match(ed) successfully, i.e.,
			// EntityRequest.
			Map: func(message *engine.Message) (*engine.Message, bool) {
				request, ok := message.Payload.(*messages.EntityRequest)
				if !ok {
					// should never happen, unless there is a bug.
					log.Warn().
						Str("entity_ids", fmt.Sprintf("%v", request.EntityIDs)).
						Hex("origin_id", logging.ID(message.OriginID)).
						Msg("cannot match the payload to entity request")
					return nil, false
				}

				message.Payload = *request // de-reference the pointer as HeroCache works with value.

				return message, true
			},
			Store: requestQueue,
		})

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:            log.With().Str("engine", "provider").Logger(),
		metrics:        metrics,
		state:          state,
		channel:        channel,
		selector:       selector,
		retrieve:       retrieve,
		requestHandler: handler,
		requestQueue:   requestQueue,
		requestChannel: make(chan *internal.EntityRequest, requestWorkers),
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(e.processQueuedRequestsShovellerWorker)
	for i := uint(0); i < requestWorkers; i++ {
		cm.AddWorker(e.processEntityRequestWorker)
	}

	e.cm = cm.Build()
	e.Component = e.cm

	return e, nil
}

// Process processes the given message from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.cm.ShutdownSignal():
		e.log.Warn().
			Hex("origin_id", logging.ID(originID)).
			Msgf("received message after shutdown")
		return nil
	default:
	}

	e.metrics.MessageReceived(e.channel.String(), metrics.MessageEntityRequest)

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

// onEntityRequest processes an entity request message from a remote node.
// Error returns:
// * NetworkTransmissionError if there is a network error happens on transmitting the requested entities.
// * InvalidInputError if the list of requested entities is invalid (empty).
// * generic error in case of unexpected failure or implementation bug.
func (e *Engine) onEntityRequest(request *internal.EntityRequest) error {
	defer e.metrics.MessageHandled(e.channel.String(), metrics.MessageEntityRequest)

	lg := e.log.With().
		Str("origin_id", request.OriginId.String()).
		Strs("entity_ids", flow.IdentifierList(request.EntityIds).Strings()).
		Logger()

	lg.Info().
		Uint64("nonce", request.Nonce).
		Msg("entity request received")

	// TODO: add reputation system to punish nodes for malicious behaviour (spam / repeated requests)

	// then, we try to get the current identity of the requester and check it against the filter
	// for the handler to make sure the requester is authorized for this resource
	requesters, err := e.state.Final().Identities(filter.And(
		e.selector,
		filter.HasNodeID[flow.Identity](request.OriginId)),
	)
	if err != nil {
		return fmt.Errorf("could not get requesters: %w", err)
	}
	if len(requesters) == 0 {
		return engine.NewInvalidInputErrorf("invalid requester origin (%x)", request.OriginId)
	}

	// try to retrieve each entity and skip missing ones
	entities := make([]flow.Entity, 0, len(request.EntityIds))
	entityIDs := make([]flow.Identifier, 0, len(request.EntityIds))
	seen := make(map[flow.Identifier]struct{})
	for _, entityID := range request.EntityIds {
		// skip requesting duplicate entity IDs
		if _, ok := seen[entityID]; ok {
			lg.Warn().
				Str("entity_id", entityID.String()).
				Bool(logging.KeySuspicious, true).
				Msg("duplicate entity ID in entity request")
			continue
		}

		entity, err := e.retrieve(entityID)
		if errors.Is(err, storage.ErrNotFound) {
			lg.Debug().
				Str("entity_id", entityID.String()).
				Msg("entity not found")
			continue
		}
		if err != nil {
			return fmt.Errorf("could not retrieve entity (%x): %w", entityID, err)
		}
		entities = append(entities, entity)
		entityIDs = append(entityIDs, entityID)
		seen[entityID] = struct{}{}
	}

	// encode all of the entities
	blobs := make([][]byte, 0, len(entities))
	for _, entity := range entities {
		blob, err := msgpack.Marshal(entity)
		if err != nil {
			return fmt.Errorf("could not encode entity (%x): %w", entity.ID(), err)
		}
		blobs = append(blobs, blob)
	}

	// NOTE: we do _NOT_ avoid sending empty responses, as this will allow
	// the requester to know we don't have any of the requested entities, which
	// allows him to retry them immediately, rather than waiting for the expiry
	// of the retry interval

	// send back the response
	res := &messages.EntityResponse{
		Nonce:     request.Nonce,
		EntityIDs: entityIDs,
		Blobs:     blobs,
	}
	err = e.con.Unicast(res, request.OriginId)
	if err != nil {
		return engine.NewNetworkTransmissionErrorf("could not send entity response: %w", err)
	}

	e.metrics.MessageSent(e.channel.String(), metrics.MessageEntityResponse)
	e.log.Info().
		Str("origin_id", request.OriginId.String()).
		Strs("entity_ids", flow.IdentifierList(entityIDs).Strings()).
		Uint64("nonce", request.Nonce). // to match with the the entity request received log
		Msg("entity response sent")

	return nil
}

func (e *Engine) processQueuedRequestsShovellerWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	e.log.Debug().Msg("process entity request shoveller worker started")

	for {
		select {
		case <-e.requestHandler.GetNotifier():
			// there is at least a single request in the queue, so we try to process it.
			e.processAvailableMessages(ctx)
		case <-ctx.Done():
			// close the internal channel, the workers will drain the channel before exiting
			close(e.requestChannel)
			e.log.Trace().Msg("processing entity request worker terminated")
			return
		}
	}
}

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

		requestEvent, ok := msg.Payload.(messages.EntityRequest)
		if !ok {
			// should never happen, as we only put EntityRequest in the queue,
			// if it does happen, it means there is a bug in the queue implementation.
			ctx.Throw(fmt.Errorf("invalid message type in entity request queue: %T", msg.Payload))
		}

		req := &internal.EntityRequest{
			OriginId:  msg.OriginID,
			EntityIds: requestEvent.EntityIDs,
			Nonce:     requestEvent.Nonce,
		}

		lg := e.log.With().
			Hex("origin_id", logging.ID(req.OriginId)).
			Str("requested_entity_ids", fmt.Sprintf("%v", req.EntityIds)).Logger()

		lg.Trace().Msg("processor is queuing entity request for processing")
		e.requestChannel <- req
		lg.Trace().Msg("processor queued up entity request for processing")
	}
}

func (e *Engine) processEntityRequestWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		request, ok := <-e.requestChannel
		if !ok {
			e.log.Trace().Msg("processing entity request worker terminated")
			return
		}
		lg := e.log.With().
			Hex("origin_id", logging.ID(request.OriginId)).
			Str("requested_entity_ids", fmt.Sprintf("%v", request.EntityIds)).Logger()
		lg.Trace().Msg("worker picked up entity request for processing")
		err := e.onEntityRequest(request)
		if err != nil {
			if engine.IsInvalidInputError(err) || engine.IsNetworkTransmissionError(err) {
				lg.Error().Err(err).Msg("worker could not process entity request")
			} else {
				// this is an unexpected error, we crash the node.
				ctx.Throw(err)
			}
		}
		lg.Trace().Msg("worker finished entity request processing")
	}
}
