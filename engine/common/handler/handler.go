package handler

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/logging"
)

// EventProcessorFunc is a function that processes an event. It is used by the AsyncEventHandler to process events.
// The first argument is the identifier of the event's origin, and the second argument is the event itself.
// The function is expected to be concurrency safe.
// Processing an event is the last step in the event's lifecycle within the AsyncEventHandler.
type EventProcessorFunc func(flow.Identifier, interface{})

// AsyncEventHandler is a component that asynchronously handles events, i.e., concurrency safe and non-blocking.
// It queues up the events and spawns a number of workers to process the events.
type AsyncEventHandler struct {
	component.Component
	cm *component.ComponentManager

	log       zerolog.Logger
	handler   *engine.MessageHandler
	store     engine.MessageStore
	processor EventProcessorFunc
}

// NewAsyncEventHandler creates a new AsyncEventHandler.
// The first argument is the logger to be used by the AsyncEventHandler.
// The second argument is the message store to be used by the AsyncEventHandler for temporarily storing events till they are processed.
// The third argument is the number of workers to be spawned by the AsyncEventHandler to pick up events from the message store and process them.
func NewAsyncEventHandler(
	log zerolog.Logger,
	store engine.MessageStore,
	workerCount uint) *AsyncEventHandler {

	h := &AsyncEventHandler{
		log:   log.With().Str("component", "async_event_handler").Logger(),
		store: store,
		handler: engine.NewMessageHandler(log, engine.NewNotifier(), engine.Pattern{
			Match: func(message *engine.Message) bool {
				return true
			},
			Store: store,
		}),
	}

	cm := component.NewComponentManagerBuilder()
	for i := uint(0); i < workerCount; i++ {
		cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			h.processEventWorker(ctx)
		})
	}

	h.cm = cm.Build()
	h.Component = h.cm

	return h
}

func (a *AsyncEventHandler) RegisterProcessor(processor EventProcessorFunc) {
	a.processor = processor
}

// processEventWorker is a worker that is spawned by the AsyncEventHandler to process events.
// The worker is blocked on the handler's notifier channel, and wakes up whenever a new event is received.
// On waking up, the worker keeps processing events till the message store is empty.
func (a *AsyncEventHandler) processEventWorker(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			a.log.Debug().Msg("processing event worker terminated")
			return
		case <-a.handler.GetNotifier():
			a.processEvents(ctx)
		}
	}
}

// processEvents is part of the worker's logic. It keeps processing events till the message store is empty.
func (a *AsyncEventHandler) processEvents(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := a.store.Get()
			if !ok {
				a.log.Trace().Msg("no more events to process, returning")
				return
			}
			lg := a.log.With().
				Hex("origin_id", logging.ID(msg.OriginID)).
				Str("payload", fmt.Sprintf("%v", msg.Payload)).Logger()
			lg.Trace().Msg("processing event")
			a.processor(msg.OriginID, msg.Payload)
			lg.Trace().Msg("event processed")
		}
	}
}

// Submit is the main entry point for the event handler. It receives an event, and asynchronously queues it for processing.
// On a happy path it returns nil. Any returned error is unexpected and indicates a bug in the code.
// It is safe to call Submit concurrently.
// It is safe to call Submit after Shutdown.
func (a *AsyncEventHandler) Submit(originId flow.Identifier, event interface{}) error {
	select {
	case <-a.cm.ShutdownSignal():
		a.log.Warn().Msg("received event after shutdown")
		return nil
	default:
	}

	err := a.handler.Process(originId, event)
	if err != nil {
		return fmt.Errorf("unexpected error while processing event: %w", err)
	}

	return nil
}
