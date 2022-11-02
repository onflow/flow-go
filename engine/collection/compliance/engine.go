package compliance

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus/sealing/counters"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// helper type used to pass originID and block through FIFO queue
type inboundBlock struct {
	originID flow.Identifier
	block    *messages.ClusterBlockProposal
}

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.ClusterBlockProposal`s
const defaultBlockQueueCapacity = 10_000

// Engine is a wrapper struct for `Core` which implements cluster consensus algorithm.
// Engine is responsible for handling incoming messages, queueing for processing, broadcasting proposals.
// Implements collection.Compliance interface.
type Engine struct {
	log                   zerolog.Logger
	metrics               module.EngineMetrics
	me                    module.Local
	headers               storage.Headers
	payloads              storage.ClusterPayloads
	state                 protocol.State
	core                  *Core
	pendingBlocks         *fifoqueue.FifoQueue // queue for processing inbound blocks
	pendingBlocksNotifier engine.Notifier
	// tracking finalized view
	finalizedView              counters.StrictMonotonousCounter
	finalizationEventsNotifier engine.Notifier

	cm *component.ComponentManager
	component.Component
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
		log:                        engineLog,
		metrics:                    core.engineMetrics,
		me:                         me,
		headers:                    core.headers,
		payloads:                   payloads,
		state:                      state,
		core:                       core,
		pendingBlocks:              blocksQueue,
		pendingBlocksNotifier:      engine.NewNotifier(),
		finalizationEventsNotifier: engine.NewNotifier(),
	}

	// create the component manager and worker threads
	eng.cm = component.NewComponentManagerBuilder().
		AddWorker(eng.processBlocksLoop).
		AddWorker(eng.finalizationProcessingLoop).
		Build()
	eng.Component = eng.cm

	return eng, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.core.hotstuff = hot
	return e
}

// WithSync adds the block requester to the engine. This must be
// called before the engine can start.
func (e *Engine) WithSync(sync module.BlockRequester) *Engine {
	e.core.sync = sync
	return e
}

func (e *Engine) Start(ctx irrecoverable.SignalerContext) {
	if e.core.hotstuff == nil {
		ctx.Throw(fmt.Errorf("must initialize compliance engine with hotstuff engine"))
	}
	if e.core.sync == nil {
		ctx.Throw(fmt.Errorf("must initialize compliance engine with sync engine"))
	}

	e.log.Info().Msg("starting hotstuff")
	e.core.hotstuff.Start(ctx)
	e.log.Info().Msg("hotstuff started")

	e.log.Info().Msg("starting compliance engine")
	e.Component.Start(ctx)
	e.log.Info().Msg("compliance engine started")
}

// Ready returns a ready channel that is closed once the engine has fully started.
// For the consensus engine, we wait for hotstuff to start.
func (e *Engine) Ready() <-chan struct{} {
	// NOTE: this will create long-lived goroutines each time Ready is called
	// Since Ready is called infrequently, that is OK. If the call frequency changes, change this code.
	return util.AllReady(e.cm, e.core.hotstuff)
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	// NOTE: this will create long-lived goroutines each time Done is called
	// Since Done is called infrequently, that is OK. If the call frequency changes, change this code.
	return util.AllDone(e.cm, e.core.hotstuff)
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
			inBlock := msg.(inboundBlock)
			err := e.core.OnBlockProposal(inBlock.originID, inBlock.block)
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
// It informs sealing.Core about finalization of the respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedView.Set(block.View) {
		e.finalizationEventsNotifier.Notify()
	}
}

// OnClusterBlockProposal feeds a new block proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnClusterBlockProposal(proposal *messages.ClusterBlockProposal) {
	e.core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageClusterBlockProposal)
	if e.pendingBlocks.Push(inboundBlock{proposal.Header.ProposerID, proposal}) {
		e.pendingBlocksNotifier.Notify()
	}
}

// OnSyncedClusterBlock feeds a block obtained from sync proposal into the processing pipeline.
// Incoming proposals are queued and eventually dispatched by worker.
func (e *Engine) OnSyncedClusterBlock(syncedBlock *events.SyncedClusterBlock) {
	e.core.engineMetrics.MessageReceived(metrics.EngineClusterCompliance, metrics.MessageSyncedClusterBlock)
	inBlock := inboundBlock{
		originID: syncedBlock.OriginID,
		block: &messages.ClusterBlockProposal{
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
