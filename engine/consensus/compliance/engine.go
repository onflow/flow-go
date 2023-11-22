package compliance

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/events"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.BlockProposal`s
const defaultBlockQueueCapacity = 10_000

// Engine is a wrapper around `compliance.Core`. The Engine queues inbound messages, relevant
// node-internal notifications, and manages the worker routines processing the inbound events,
// and forwards outbound messages to the networking layer.
// `compliance.Core` implements the actual compliance logic.
// Implements consensus.Compliance interface.
type Engine struct {
	component.Component
	hotstuff.FinalizationConsumer

	log                   zerolog.Logger
	mempoolMetrics        module.MempoolMetrics
	engineMetrics         module.EngineMetrics
	me                    module.Local
	headers               storage.Headers
	payloads              storage.Payloads
	tracer                module.Tracer
	state                 protocol.State
	core                  *Core
	pendingBlocks         *fifoqueue.FifoQueue // queue for processing inbound blocks
	pendingBlocksNotifier engine.Notifier
}

var _ consensus.Compliance = (*Engine)(nil)

func NewEngine(
	log zerolog.Logger,
	me module.Local,
	core *Core,
) (*Engine, error) {

	// Inbound FIFO queue for `messages.BlockProposal`s
	blocksQueue, err := fifoqueue.NewFifoQueue(
		defaultBlockQueueCapacity,
		fifoqueue.WithLengthObserver(func(len int) { core.mempoolMetrics.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound block proposals: %w", err)
	}

	eng := &Engine{
		log:                   log.With().Str("compliance", "engine").Logger(),
		me:                    me,
		mempoolMetrics:        core.mempoolMetrics,
		engineMetrics:         core.engineMetrics,
		headers:               core.headers,
		payloads:              core.payloads,
		pendingBlocks:         blocksQueue,
		state:                 core.state,
		tracer:                core.tracer,
		core:                  core,
		pendingBlocksNotifier: engine.NewNotifier(),
	}
	finalizationActor, finalizationWorker := events.NewFinalizationActor(eng.processOnFinalizedBlock)
	eng.FinalizationConsumer = finalizationActor
	// create the component manager and worker threads
	eng.Component = component.NewComponentManagerBuilder().
		AddWorker(eng.processBlocksLoop).
		AddWorker(finalizationWorker).
		Build()

	return eng, nil
}

// processBlocksLoop processes available block, vote, and timeout messages as they are queued.
func (e *Engine) processBlocksLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newMessageSignal := e.pendingBlocksNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-newMessageSignal:
			err := e.processQueuedBlocks(doneSignal) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// processQueuedBlocks processes any available messages until the message queue is empty.
// Only returns when all inbound queues are empty (or the engine is terminated).
// No errors are expected during normal operation. All returned exceptions are potential
// symptoms of internal state corruption and should be fatal.
func (e *Engine) processQueuedBlocks(doneSignal <-chan struct{}) error {
	for {
		select {
		case <-doneSignal:
			return nil
		default:
		}

		msg, ok := e.pendingBlocks.Pop()
		if ok {
			batch := msg.(flow.Slashable[[]*messages.BlockProposal])
			for _, block := range batch.Message {
				err := e.core.OnBlockProposal(flow.Slashable[*messages.BlockProposal]{
					OriginID: batch.OriginID,
					Message:  block,
				})
				e.core.engineMetrics.MessageHandled(metrics.EngineCompliance, metrics.MessageBlockProposal)
				if err != nil {
					return fmt.Errorf("could not handle block proposal: %w", err)
				}
			}
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// OnBlockProposal feeds a new block proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnBlockProposal(proposal flow.Slashable[*messages.BlockProposal]) {
	e.core.engineMetrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
	proposalAsList := flow.Slashable[[]*messages.BlockProposal]{
		OriginID: proposal.OriginID,
		Message:  []*messages.BlockProposal{proposal.Message},
	}
	if e.pendingBlocks.Push(proposalAsList) {
		e.pendingBlocksNotifier.Notify()
	} else {
		e.core.engineMetrics.InboundMessageDropped(metrics.EngineCompliance, metrics.MessageBlockProposal)
	}
}

// OnSyncedBlocks feeds a batch of blocks obtained via sync into the processing pipeline.
// Blocks in batch aren't required to be in any particular order.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal]) {
	e.core.engineMetrics.MessageReceived(metrics.EngineCompliance, metrics.MessageSyncedBlocks)
	if e.pendingBlocks.Push(blocks) {
		e.pendingBlocksNotifier.Notify()
	} else {
		e.core.engineMetrics.InboundMessageDropped(metrics.EngineCompliance, metrics.MessageSyncedBlocks)
	}
}

// processOnFinalizedBlock informs compliance.Core about finalization of the respective block.
// The input to this callback is treated as trusted. This method should be executed on
// `OnFinalizedBlock` notifications from the node-internal consensus instance.
// No errors expected during normal operations.
func (e *Engine) processOnFinalizedBlock(block *model.Block) error {
	// retrieve the latest finalized header, so we know the height
	finalHeader, err := e.headers.ByBlockID(block.BlockID)
	if err != nil { // no expected errors
		return fmt.Errorf("could not get finalized header: %w", err)
	}
	e.core.ProcessFinalizedBlock(finalHeader)
	return nil
}
