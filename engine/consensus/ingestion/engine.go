// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"fmt"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
)

const defaultGuaranteeQueueCapacity = 1000

const defaultIngestionEngineWorkers = 3

// Engine represents the ingestion engine, used to funnel collections from a
// cluster of collection nodes to the set of consensus nodes. It represents the
// link between collection nodes and consensus nodes and has a counterpart with
// the same engine ID in the collection node.
type Engine struct {
	*component.ComponentManager
	log               zerolog.Logger          // used to log relevant actions with context
	tracer            module.Tracer           // used for tracing
	metrics           module.EngineMetrics    // used to track sent & received messages
	mempool           module.MempoolMetrics   // used to track mempool metrics
	spans             module.ConsensusMetrics // used to track consensus spans
	me                module.Local            // used to access local node information
	con               network.Conduit         // conduit to receive/send guarantees
	pendingGuarantees engine.MessageStore
	messageHandler    *engine.MessageHandler
}

// New creates a new collection propagation engine.
func New(
	log zerolog.Logger,
	tracer module.Tracer,
	metrics module.EngineMetrics,
	spans module.ConsensusMetrics,
	mempool module.MempoolMetrics,
	net network.Network,
	me module.Local,
) (*Engine, error) {

	//guaranteesQueue, err := fifoqueue.NewFifoQueue(
	//	fifoqueue.WithCapacity(defaultGuaranteeQueueCapacity),
	//)

	//handler := engine.NewMessageHandler(
	//	log.With().Str("ingestion", "engine").Logger(),
	//	engine.NewNotifier(),
	//	engine.Pattern{
	//		Match: func(msg *engine.Message) bool {
	//			_, ok := msg.Payload.(*messages.BlockProposal)
	//			if ok {
	//				core.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
	//			}
	//			return ok
	//		},
	//		Store: pendingBlocks,
	//	},

	// initialize the propagation engine with its dependencies
	e := &Engine{
		log:     log.With().Str("engine", "ingestion").Logger(),
		tracer:  tracer,
		metrics: metrics,
		mempool: mempool,
		spans:   spans,
		me:      me,
	}

	componentManagerBuilder := component.NewComponentManagerBuilder()

	for i := 0; i < defaultIngestionEngineWorkers; i++ {
		componentManagerBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			err := e.loop()
			ctx.Throw(err)
		})
	}

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
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(_ network.Channel, originID flow.Identifier, event interface{}) error {
	return e.messageHandler.Process(originID, event)
}

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.CollectionGuarantee:
		e.metrics.MessageReceived(metrics.EngineConsensusIngestion, metrics.MessageCollectionGuarantee)
		err := e.onGuarantee(originID, ev)
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
			return err
		}
		return nil
	default:
		return fmt.Errorf("input with incompatible type %T: %w", event, engine.IncompatibleInputTypeError)
	}
}

func (e *Engine) processAvailableMessages() error {

}

func (e *Engine) loop() error {

}
