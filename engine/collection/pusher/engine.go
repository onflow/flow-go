// Package pusher implements an engine for providing access to resources held
// by the collection node, including collections, collection guarantees, and
// transactions.
package pusher

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
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

// Engine is part of the Collection Node. It broadcasts finalized collections
// ("collection guarantees") that the cluster generates to Consensus Nodes
// for inclusion in blocks.
type Engine struct {
	log          zerolog.Logger
	engMetrics   module.EngineMetrics
	conduit      network.Conduit
	me           module.Local
	state        protocol.State
	collections  storage.Collections
	transactions storage.Transactions

	notifier engine.Notifier
	queue    *fifoqueue.FifoQueue

	component.Component
	cm *component.ComponentManager
}

// TODO convert to network.MessageProcessor
var _ network.Engine = (*Engine)(nil)
var _ component.Component = (*Engine)(nil)

// New creates a new pusher engine.
func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	state protocol.State,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	me module.Local,
	collections storage.Collections,
	transactions storage.Transactions,
) (*Engine, error) {
	queue, err := fifoqueue.NewFifoQueue(
		200, // roughly 1 minute of collections, at 3BPS
		fifoqueue.WithLengthObserver(func(len int) {
			mempoolMetrics.MempoolEntries(metrics.ResourceSubmitCollectionGuaranteesQueue, uint(len))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create fifoqueue: %w", err)
	}

	notifier := engine.NewNotifier()

	e := &Engine{
		log:          log.With().Str("engine", "pusher").Logger(),
		engMetrics:   engMetrics,
		me:           me,
		state:        state,
		collections:  collections,
		transactions: transactions,

		notifier: notifier,
		queue:    queue,
	}

	conduit, err := net.Register(channels.PushGuarantees, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for push protocol: %w", err)
	}
	e.conduit = conduit

	e.cm = component.NewComponentManagerBuilder().
		AddWorker(e.outboundQueueWorker).
		Build()
	e.Component = e.cm

	return e, nil
}

// outboundQueueWorker implements a component worker which broadcasts collection guarantees,
// enqueued by the Finalizer upon finalization, to Consensus Nodes.
func (e *Engine) outboundQueueWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	wake := e.notifier.Channel()
	for {
		select {
		case <-done:
			return
		case <-wake:
			err := e.processOutboundMessages(ctx)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// processOutboundMessages processes any available messages from the queue.
// Only returns when the queue is empty (or the engine is terminated).
// No errors expected during normal operations.
func (e *Engine) processOutboundMessages(ctx context.Context) error {
	for {
		nextMessage, ok := e.queue.Pop()
		if !ok {
			return nil
		}

		asSCGMsg, ok := nextMessage.(*messages.SubmitCollectionGuarantee)
		if !ok {
			return fmt.Errorf("invalid message type in pusher engine queue")
		}

		err := e.publishCollectionGuarantee(&asSCGMsg.Guarantee)
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	ev, ok := event.(*messages.SubmitCollectionGuarantee)
	if ok {
		e.SubmitCollectionGuarantee(ev)
	} else {
		engine.LogError(e.log, fmt.Errorf("invalid message argument to pusher engine"))
	}
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel channels.Channel, originID flow.Identifier, event interface{}) {
	engine.LogError(e.log, fmt.Errorf("pusher engine should only receive local messages on the same node"))
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	ev, ok := event.(*messages.SubmitCollectionGuarantee)
	if ok {
		e.SubmitCollectionGuarantee(ev)
		return nil
	} else {
		return fmt.Errorf("invalid message argument to pusher engine")
	}
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, message any) error {
	return fmt.Errorf("pusher engine should only receive local messages on the same node")
}

// SubmitCollectionGuarantee adds a collection guarantee to the engine's queue
// to later be published to consensus nodes.
func (e *Engine) SubmitCollectionGuarantee(msg *messages.SubmitCollectionGuarantee) {
	ok := e.queue.Push(msg)
	if !ok {
		engine.LogError(e.log, fmt.Errorf("failed to store collection guarantee in queue"))
		return
	}
	e.notifier.Notify()
}

// publishCollectionGuarantee publishes the collection guarantee to all consensus nodes.
// No errors expected during normal operation.
func (e *Engine) publishCollectionGuarantee(guarantee *flow.CollectionGuarantee) error {
	consensusNodes, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("could not get consensus nodes: %w", err)
	}

	// NOTE: Consensus nodes do not broadcast guarantees among themselves, so it needs that
	// at least one collection node make a publish to all of them.
	err = e.conduit.Publish(guarantee, consensusNodes.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit collection guarantee: %w", err)
	}

	e.engMetrics.MessageSent(metrics.EngineCollectionProvider, metrics.MessageCollectionGuarantee)

	e.log.Debug().
		Hex("guarantee_id", logging.ID(guarantee.ID())).
		Hex("ref_block_id", logging.ID(guarantee.ReferenceBlockID)).
		Msg("submitting collection guarantee")

	return nil
}
