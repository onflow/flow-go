package handler

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

type EventHandlerFunc func(flow.Identifier, interface{})

// AsyncEventHandler is a component that asynchronously handles events, i.e., concurrency safe and non-blocking.
// It queues up the events and spawns a number of workers to process the events.
type AsyncEventHandler struct {
	component.Component
	cm *component.ComponentManager

	log             zerolog.Logger
	handler         *engine.MessageHandler
	queue           engine.MessageStore
	msgChannel      chan *engine.Message
	externalHandler EventHandlerFunc
}

func NewAsyncEventHandler(
	log zerolog.Logger,
	handler EventHandlerFunc,
	queueSize uint32,
	workerCount uint) *AsyncEventHandler {

	q := queue.NewHeroStore(queueSize, unittest.Logger(), metrics.NewNoopCollector())

	n := &AsyncEventHandler{
		log:   log.With().Str("component", "async_event_handler").Logger(),
		queue: q,
		handler: engine.NewMessageHandler(log, engine.NewNotifier(), engine.Pattern{
			Store: q,
		}),
		externalHandler: handler,
	}

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(n.processEventShovlerWorker)
	for i := uint(0); i < workerCount; i++ {
		cm.AddWorker(n.processEventWorker)
	}

	n.cm = cm.Build()
	n.Component = n.cm

	return n
}

// processEventShovlerWorker is constantly listening on the MessageHandler for new events,
// and pushes new events into the request channel to be picked by workers.
func (n *AsyncEventHandler) processEventShovlerWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	n.log.Debug().Msg("process event shovler started")

	for {
		select {
		case <-n.handler.GetNotifier():
			// there is at least a single event to process.
			n.processAvailableEvents(ctx)
		case <-ctx.Done():
			// close the internal channel, the workers will drain the channel before exiting
			close(n.msgChannel)
			n.log.Debug().Msg("processing event worker terminated")
			return
		}
	}
}

// processAvailableEvents processes all available events in the queue.
func (n *AsyncEventHandler) processAvailableEvents(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, ok := n.queue.Get()
		if !ok {
			// no more events to process
			return
		}

		n.log.Trace().Msg("shovler is queuing messages for processing")
		n.msgChannel <- msg
		n.log.Trace().Msg("shovler queued up messages for processing")
	}
}

// processEventWorker is a worker that processes events from the request channel.
func (n *AsyncEventHandler) processEventWorker(_ irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		msg, ok := <-n.msgChannel
		if !ok {
			n.log.Trace().Msg("processing event worker terminated")
			return
		}
		n.externalHandler(msg.OriginID, msg.Payload)
	}
}

// Process is the main entry point for the event handler. It receives an event, and asynchronously queues it for processing.
func (n *AsyncEventHandler) Process(originId flow.Identifier, event interface{}) error {
	select {
	case <-n.cm.ShutdownSignal():
		n.log.Warn().Msg("received event after shutdown")
		return nil
	default:
	}

	err := n.handler.Process(originId, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			n.log.Warn().Msgf("%v delivered unsupported message %T through %v", originId)
			return nil
		}
		return fmt.Errorf("unexpected error while processing event: %w", err)
	}

	return nil
}
