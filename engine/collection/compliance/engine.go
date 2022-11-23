package compliance

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.ClusterBlockProposal`s
const defaultBlockQueueCapacity = 10_000

// Engine is a wrapper struct for `Core` which implements cluster consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
// Implements collection.Compliance interface.
type Engine struct {
	*component.ComponentManager
	log                    zerolog.Logger
	metrics                module.EngineMetrics
	me                     module.Local
	headers                storage.Headers
	payloads               storage.ClusterPayloads
	state                  protocol.State
	core                   *Core
	pendingBlocks          *fifoqueue.FifoQueue // queue for processing inbound blocks
	pendingBlocksNotifier  engine.Notifier
	finalizedBlockTracker  *tracker.NewestBlockTracker
	finalizedBlockNotifier engine.Notifier
}

var _ collection.Compliance = (*Engine)(nil)

func NewEngine(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	payloads storage.ClusterPayloads,
	core *Core,
) (*Engine, error) {
	engineLog := log.With().Str("cluster_compliance", "engine").Logger()

	// FIFO queue for block proposals
	blocksQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) {
			core.mempoolMetrics.MempoolEntries(metrics.ResourceClusterBlockProposalQueue, uint(len))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound block proposals: %w", err)
	}

	eng := &Engine{
		log:                    engineLog,
		metrics:                core.engineMetrics,
		me:                     me,
		headers:                core.headers,
		payloads:               payloads,
		state:                  state,
		core:                   core,
		pendingBlocks:          blocksQueue,
		pendingBlocksNotifier:  engine.NewNotifier(),
		finalizedBlockTracker:  tracker.NewNewestBlockTracker(),
		finalizedBlockNotifier: engine.NewNotifier(),
	}

	// create the component manager and worker threads
	eng.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(eng.processBlocksLoop).
		AddWorker(eng.finalizationProcessingLoop).
		Build()

	return eng, nil
}

// processBlocksLoop processes available blocks as they are queued.
func (e *Engine) processBlocksLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newMessageSignal := e.pendingBlocksNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-newMessageSignal:
			err := e.processQueuedBlocks(doneSignal)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// processQueuedBlocks processes any available messages from the inbound queues.
// Only returns when all inbound queues are empty (or the engine is terminated).
// No errors expected during normal operations.
func (e *Engine) processQueuedBlocks(doneSignal <-chan struct{}) error {
	for {
		select {
		case <-doneSignal:
			return nil
		default:
		}

		msg, ok := e.pendingBlocks.Pop()
		if ok {
			inBlock := msg.(flow.Slashable[messages.ClusterBlockProposal])
			err := e.core.OnBlockProposal(inBlock.OriginID, inBlock.Message)
			if err != nil {
				return fmt.Errorf("could not handle block proposal: %w", err)
			}
			continue
		}

		// when there is no more messages in the queue, back to the loop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs compliance.Core about finalization of the respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedBlockTracker.Track(block) {
		e.finalizedBlockNotifier.Notify()
	}
}

// OnClusterBlockProposal feeds a new block proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnClusterBlockProposal(proposal flow.Slashable[messages.ClusterBlockProposal]) {
	e.core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
	if e.pendingBlocks.Push(proposal) {
		e.pendingBlocksNotifier.Notify()
	}
}

// OnSyncedClusterBlock feeds a block obtained from sync proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnSyncedClusterBlock(syncedBlock flow.Slashable[messages.ClusterBlockProposal]) {
	e.core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageSyncedClusterBlock)
	if e.pendingBlocks.Push(syncedBlock) {
		e.pendingBlocksNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	blockFinalizedSignal := e.finalizedBlockNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-blockFinalizedSignal:
			// retrieve the latest finalized header, so we know the height
			finalHeader, err := e.headers.ByBlockID(e.finalizedBlockTracker.NewestBlock().BlockID)
			if err != nil { // no expected errors
				ctx.Throw(err)
			}
			e.core.ProcessFinalizedBlock(finalHeader)
		}
	}
}
