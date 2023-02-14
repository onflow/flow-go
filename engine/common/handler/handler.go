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
)

type EventHandlerFunc func(flow.Identifier, interface{})

// AsyncEventHandler is a component that asynchronously handles events, i.e., concurrency safe and non-blocking.
// It queues up the events and spawns a number of workers to process the events.
type AsyncEventHandler struct {
	component.Component
	cm *component.ComponentManager

	log       zerolog.Logger
	handler   *engine.MessageHandler
	queue     engine.MessageStore
	processor EventHandlerFunc
}

func NewAsyncEventHandler(
	log zerolog.Logger,
	queueSize uint32,
	workerCount uint) *AsyncEventHandler {

	q := queue.NewHeroStore(queueSize, log, metrics.NewNoopCollector())

	n := &AsyncEventHandler{
		log:   log.With().Str("component", "async_event_handler").Logger(),
		queue: q,
		handler: engine.NewMessageHandler(log, engine.NewNotifier(), engine.Pattern{
			Match: func(message *engine.Message) bool {
				return true
			},
			Store: q,
		}),
	}

	cm := component.NewComponentManagerBuilder()
	for i := uint(0); i < workerCount; i++ {
		cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			n.processEventWorker(ctx)
		})
	}

	n.cm = cm.Build()
	n.Component = n.cm

	return n
}

func (n *AsyncEventHandler) RegisterProcessor(processor EventHandlerFunc) {
	n.processor = processor
}

// processEventWorker is a worker that processes events from the request channel.
func (n *AsyncEventHandler) processEventWorker(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			n.log.Debug().Msg("processing event worker terminated")
			return
		case <-n.handler.GetNotifier():
			n.processMessages(ctx)
		}
	}
}

func (n *AsyncEventHandler) processMessages(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := n.queue.Get()
			if !ok {
				// no more events to process
				return
			}
			n.processor(msg.OriginID, msg.Payload)
		}
	}
}

// Submit is the main entry point for the event handler. It receives an event, and asynchronously queues it for processing.
// On a happy path it returns nil. Any returned error is unexpected and indicates a bug in the code.
// It is safe to call Submit concurrently.
// It is safe to call Submit after Shutdown.
func (n *AsyncEventHandler) Submit(originId flow.Identifier, event interface{}) error {
	select {
	case <-n.cm.ShutdownSignal():
		n.log.Warn().Msg("received event after shutdown")
		return nil
	default:
	}

	err := n.handler.Process(originId, event)
	if err != nil {
		return fmt.Errorf("unexpected error while processing event: %w", err)
	}

	return nil
}
