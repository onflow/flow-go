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

var _ network.MessageProcessor = (*Engine)(nil)
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

	e := &Engine{
		log:          log.With().Str("engine", "pusher").Logger(),
		engMetrics:   engMetrics,
		me:           me,
		state:        state,
		collections:  collections,
		transactions: transactions,

		notifier: engine.NewNotifier(),
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
		item, ok := e.queue.Pop()
		if !ok {
			return nil
		}

		guarantee, ok := item.(*flow.CollectionGuarantee)
		if !ok {
			return fmt.Errorf("invalid type in pusher engine queue")
		}

		err := e.publishCollectionGuarantee(guarantee)
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

// Process processes the given event from the node with the given origin ID in
// a non-blocking manner. It returns the potential processing error when done.
// Because the pusher engine does not accept inputs from the network,
// always drop any messages and return an error.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, message any) error {
	return fmt.Errorf("pusher engine should only receive local messages on the same node: got message %T on channel %v from origin %v", message, channel, originID)
}

// SubmitCollectionGuarantee adds a collection guarantee to the engine's queue
// to later be published to consensus nodes.
func (e *Engine) SubmitCollectionGuarantee(guarantee *flow.CollectionGuarantee) {
	if e.queue.Push(guarantee) {
		e.notifier.Notify()
	} else {
		e.engMetrics.OutboundMessageDropped(metrics.EngineCollectionProvider, metrics.MessageCollectionGuarantee)
	}
}

// publishCollectionGuarantee publishes the collection guarantee to all consensus nodes.
// No errors expected during normal operation.
func (e *Engine) publishCollectionGuarantee(guarantee *flow.CollectionGuarantee) error {
	consensusNodes, err := e.state.Final().Identities(filter.HasRole[flow.Identity](flow.RoleConsensus))
	if err != nil {
		return fmt.Errorf("could not get consensus nodes' identities: %w", err)
	}

	// NOTE: Consensus nodes do not broadcast guarantees among themselves. So for the collection to be included,
	// at least one collector has to successfully broadcast the collection to consensus nodes. Otherwise, the
	// collection is lost, which is acceptable as long as we only lose a small fraction of collections.
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
