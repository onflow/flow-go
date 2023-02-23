package handler

import (
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// ErrSubmissionFailed is returned when the AsyncEventDistributor fails to submit an event to the message store.
// For example, this error is returned when the message store is full, or when there is already the same event in the message store.
var ErrSubmissionFailed = errors.New("could not submit event")

// EventConsumer is an interface that defines the ConsumeEvent method.
// This interface models the consumer of an event.
// The ConsumeEvent method will be called by the AsyncEventDistributor when a worker picks up an event from the message store and
// distributes it to the consumer.
type EventConsumer[T any] interface {
	ConsumeEvent(event T)
}

// AsyncEventDistributor is a component that asynchronously distributes events to registered consumers.
// It acts as a broker between the producer and the consumers.
// The producer can submit events to the AsyncEventDistributor in a non-blocking manner,
// i.e., it will not wait for the event to be distributed to all the consumers.
// The AsyncEventDistributor will store the events in a message store and spawn a number of workers to process the events.
// The workers will pick up events from the message store and distribute them to all the consumers.
// Each worker acts blocking on event distribution to all the consumers, i.e., it will not pick up the next event from the message store
// until all the consumers have processed the current event.
// Note that the AsyncEventDistributor does not guarantee that the events will be processed in the same order as they are submitted.
// The order of processing is determined by two factors: (1) the order in which the events are picked up from the message store, and
// (2) the number of workers.
// When there is only one worker and a FIFO message store is used, the events will be picked AND processed in the same order as they are submitted.
// When there are multiple workers and a FIFO message store is used, the events will be picked up in the same order as they are submitted, but there is
// no guarantee that they will be processed in the same order.
// With a non-FIFO message store, the order of processing is not guaranteed to be the same as the order of submission.
type AsyncEventDistributor[T any] struct {
	component.Component
	cm *component.ComponentManager

	log zerolog.Logger
	// store is the message store used by the AsyncEventDistributor to temporarily store events till they are processed.
	store engine.MessageStore

	// consumerLock protects the consumer field from concurrent updates.
	consumerLock sync.RWMutex
	// consumers is the list of consumers that will be notified when a new event is submitted.
	consumers []EventConsumer[T]
	notifier  engine.Notifier
}

// NewAsyncEventDistributor creates a new AsyncEventDistributor.
// The first argument is the logger to be used by the AsyncEventDistributor.
// The second argument is the message store to be used by the AsyncEventDistributor for temporarily storing events till they are processed.
// The third argument is the number of workers to be spawned by the AsyncEventDistributor to pick up events from the message store and process them.
func NewAsyncEventDistributor[T any](
	log zerolog.Logger,
	store engine.MessageStore,
	workerCount uint,
) *AsyncEventDistributor[T] {
	h := &AsyncEventDistributor[T]{
		log:       log.With().Str("component", "async_event_distributor").Logger(),
		store:     store,
		notifier:  engine.NewNotifier(),
		consumers: make([]EventConsumer[T], 0),
	}

	cm := component.NewComponentManagerBuilder()
	for i := uint(0); i < workerCount; i++ {
		cm.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()

			h.distributorWorker(ctx)
		})
	}

	h.cm = cm.Build()
	h.Component = h.cm

	return h
}

// RegisterConsumer registers a new consumer with the AsyncEventDistributor.
// The consumer will be notified whenever a new event is distributed by the AsyncEventDistributor.
// Registering a consumer can be done at any time, even after the AsyncEventDistributor has been started.
// However, the consumer will not be notified of events that were submitted before the consumer was registered.
func (a *AsyncEventDistributor[T]) RegisterConsumer(consumer EventConsumer[T]) {
	a.consumerLock.Lock()
	defer a.consumerLock.Unlock()

	a.consumers = append(a.consumers, consumer)
}

// distributorWorker is a worker that is spawned by the AsyncEventDistributor to pick up events from the message store and
// distribute them to all the consumers.
// The worker is blocked on the distributor's notifier channel, and wakes up whenever a new event is received.
// On waking up, the worker keeps distributing events till the message store is empty.
func (a *AsyncEventDistributor[T]) distributorWorker(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			a.log.Debug().Msg("distributing event worker terminated")
			return
		case <-a.notifier.Channel():
			a.distributeEvents(ctx)
		}
	}
}

// distributeEvents is part of the worker's logic. It keeps distributing events till the message store is empty.
func (a *AsyncEventDistributor[T]) distributeEvents(ctx irrecoverable.SignalerContext) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, ok := a.store.Get()
			if !ok {
				a.log.Trace().Msg("no more events to distribute, returning")
				return
			}
			event, ok := msg.Payload.(T)
			if !ok {
				a.log.Fatal().Msgf("invalid event type expected: %T", msg.Payload)
			}
			lg := a.log.With().
				Str("payload", fmt.Sprintf("%v", msg.Payload)).Logger()
			lg.Trace().Msg("distributing event to consumers started")
			a.distributeEvent(event)
			lg.Trace().Msg("event distributed to all registered consumers")
		}
	}
}

// distributeEvent sends the event to all registered consumers. It is concurrency safe and blocks until all consumers have processed the event.
func (a *AsyncEventDistributor[T]) distributeEvent(event T) {
	a.consumerLock.RLock()
	defer a.consumerLock.RUnlock()
	for _, consumer := range a.consumers {
		consumer.ConsumeEvent(event)
	}
}

// Submit is the main entry point for the AsyncEventDistributor.
// It receives an event, and asynchronously queues it for distribution.
// The event submission is blocking, but the actual distribution is asynchronous and non-blocking. The event is queued up
// in the message store, and the distributor worker picks it up from there and distributes it to all the consumers.
// On a happy path Submit returns nil. It returns ErrSubmissionFailed if the event could not be queued up for processing, e.g.,
// due to the message store being full.
// It is safe to call Submit concurrently.
// It is safe to call Submit after Shutdown.
func (a *AsyncEventDistributor[T]) Submit(event T) error {
	select {
	case <-a.cm.ShutdownSignal():
		a.log.Warn().Msg("received event after shutdown")
		return nil
	default:
	}

	ok := a.store.Put(&engine.Message{
		Payload: event,
	})

	if !ok {
		return ErrSubmissionFailed
	}

	a.notifier.Notify()
	return nil
}
