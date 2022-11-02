package compliance

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type inboundBlock struct {
	originID flow.Identifier
	block    *messages.BlockProposal
}

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.BlockProposal`s
const defaultBlockQueueCapacity = 10_000

// Engine is a wrapper around `compliance.Core`. The Engine queues inbound messages, relevant
// node-internal notifications, and manages the worker routines processing the inbound events,
// and forwards outbound messages to the networking layer.
// `compliance.Core` implements the actual compliance logic.
// Implements consensus.Compliance interface.
type Engine struct {
	log                   zerolog.Logger
	mempoolMetrics        module.MempoolMetrics
	engineMetrics         module.EngineMetrics
	me                    module.Local
	headers               storage.Headers
	payloads              storage.Payloads
	tracer                module.Tracer
	state                 protocol.State
	core                  *Core
	pendingBlocks         *fifoqueue.FifoQueue // queues for processing inbound blocks
	pendingBlocksNotifier engine.Notifier
	// tracking finalized view
	finalizedView              counters.StrictMonotonousCounter
	finalizationEventsNotifier engine.Notifier

	*component.ComponentManager
}

var _ consensus.Compliance = (*Engine)(nil)

func NewEngine(
	log zerolog.Logger,
	me module.Local,
	core *Core,
) (*Engine, error) {

	// Inbound FIFO queue for `messages.BlockProposal`s
	blocksQueue, err := fifoqueue.NewFifoQueue(
		fifoqueue.WithCapacity(defaultBlockQueueCapacity),
		fifoqueue.WithLengthObserver(func(len int) { core.mempoolMetrics.MempoolEntries(metrics.ResourceBlockProposalQueue, uint(len)) }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound block proposals: %w", err)
	}

	eng := &Engine{
		log:                        log.With().Str("compliance", "engine").Logger(),
		me:                         me,
		mempoolMetrics:             core.mempoolMetrics,
		engineMetrics:              core.engineMetrics,
		headers:                    core.headers,
		payloads:                   core.payloads,
		pendingBlocks:              blocksQueue,
		state:                      core.state,
		tracer:                     core.tracer,
		core:                       core,
		pendingBlocksNotifier:      engine.NewNotifier(),
		finalizationEventsNotifier: engine.NewNotifier(),
	}

	// create the component manager and worker threads
	eng.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(eng.processBlocksLoop).
		AddWorker(eng.finalizationProcessingLoop).
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
			err := e.processQueuedBlocks() // no errors expected during normal operations
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
func (e *Engine) processQueuedBlocks() error {
	for {
		msg, ok := e.pendingBlocks.Pop()
		if ok {
			inBlock := msg.(inboundBlock)
			err := e.core.OnBlockProposal(inBlock.originID, inBlock.block)
			if err != nil {
				return fmt.Errorf("could not handle block proposal: %w", err)
			}
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs sealing.Core about finalization of respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedView.Set(block.View) {
		e.finalizationEventsNotifier.Notify()
	}
}

// OnBlockProposal feeds a new block proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnBlockProposal(proposal *messages.BlockProposal) {
	e.core.engineMetrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
	if e.pendingBlocks.Push(inboundBlock{proposal.Header.ProposerID, proposal}) {
		e.pendingBlocksNotifier.Notify()
	}
}

// OnSyncedBlock feeds a block obtained from sync proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnSyncedBlock(syncedBlock *events.SyncedBlock) {
	e.core.engineMetrics.MessageReceived(metrics.EngineCompliance, metrics.MessageSyncedBlock)
	inBlock := inboundBlock{
		originID: syncedBlock.OriginID,
		block: &messages.BlockProposal{
			Payload: syncedBlock.Block.Payload,
			Header:  syncedBlock.Block.Header,
		},
	}
	if e.pendingBlocks.Push(inBlock) {
		e.pendingBlocksNotifier.Notify()
	}
}

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events
func (e *Engine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	blockFinalizedSignal := e.finalizationEventsNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-blockFinalizedSignal:
			e.core.ProcessFinalizedView(e.finalizedView.Value())
		}
	}
}
