// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
)

// defaultGuaranteeQueueCapacity maximum capacity of pending events queue, everything above will be dropped
const defaultGuaranteeQueueCapacity = 1000

// defaultIngestionEngineWorkers number of goroutines engine will use for processing events
const defaultIngestionEngineWorkers = 3

// Engine represents the ingestion engine, used to funnel collections from a
// cluster of collection nodes to the set of consensus nodes. It represents the
// link between collection nodes and consensus nodes and has a counterpart with
// the same engine ID in the collection node.
type Engine struct {
	*component.ComponentManager
	log               zerolog.Logger         // used to log relevant actions with context
	me                module.Local           // used to access local node information
	con               network.Conduit        // conduit to receive/send guarantees
	core              *Core                  // core logic of processing guarantees
	pendingGuarantees engine.MessageStore    // message store of pending events
	messageHandler    *engine.MessageHandler // message handler for incoming events
}

// New creates a new collection propagation engine.
func New(
	log zerolog.Logger,
	engineMetrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	core *Core,
) (*Engine, error) {

	logger := log.With().Str("ingestion", "engine").Logger()

	guaranteesQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultGuaranteeQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempool.MempoolEntries(metrics.ResourceCollectionGuaranteesQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create guarantees queue: %w", err)
	}

	pendingGuarantees := &engine.FifoMessageStore{
		FifoQueue: guaranteesQueue,
	}

	handler := engine.NewMessageHandler(
		logger,
		engine.NewNotifier(),
		engine.Pattern{
			Match: func(msg *engine.Message) bool {
				_, ok := msg.Payload.(*flow.CollectionGuarantee)
				if ok {
					engineMetrics.MessageReceived(metrics.EngineConsensusIngestion, metrics.MessageCollectionGuarantee)
				}
				return ok
			},
			Store: pendingGuarantees,
		},
	)

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:               logger,
		me:                me,
		core:              core,
		pendingGuarantees: pendingGuarantees,
		messageHandler:    handler,
	}

	componentManagerBuilder := component.NewComponentManagerBuilder()

	for i := 0; i < defaultIngestionEngineWorkers; i++ {
		componentManagerBuilder.AddWorker("main", func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc, lookup component.LookupFunc) {
			ready()
			err := e.loop(ctx)
			if err != nil {
				ctx.Throw(err)
			}
		})
	}

	e.ComponentManager = componentManagerBuilder.Build()

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ReceiveGuarantees, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con
	return e, nil
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	err := e.ProcessLocal(event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	err := e.Process(channel, originID, event)
	if err != nil {
		e.log.Fatal().Err(err).Msg("internal error processing event")
	}
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.messageHandler.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns error only in unexpected scenario.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	err := e.messageHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}
	return nil
}

// processAvailableMessages processes the given ingestion engine event.
func (e *Engine) processAvailableMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default: // fall through to business logic
		}

		msg, ok := e.pendingGuarantees.Get()
		if ok {
			originID := msg.OriginID
			err := e.core.OnGuarantee(originID, msg.Payload.(*flow.CollectionGuarantee))
			if err != nil {
				if engine.IsInvalidInputError(err) {
					e.log.Error().Str("origin", originID.String()).Err(err).Msg("received invalid collection guarantee")
					return nil
				}
				if engine.IsOutdatedInputError(err) {
					e.log.Warn().Str("origin", originID.String()).Err(err).Msg("received outdated collection guarantee")
					return nil
				}
				if engine.IsUnverifiableInputError(err) {
					e.log.Warn().Str("origin", originID.String()).Err(err).Msg("received unverifiable collection guarantee")
					return nil
				}
				return fmt.Errorf("processing collection guarantee unexpected err: %w", err)
			}

			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

func (e *Engine) loop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-e.messageHandler.GetNotifier():
			err := e.processAvailableMessages(ctx)
			if err != nil {
				return fmt.Errorf("internal error processing queued message: %w", err)
			}
		}
	}
}
