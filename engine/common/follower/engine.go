package follower

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/storage"
)

type EngineOption func(*Engine)

// WithChannel sets the channel the follower engine will use to receive blocks.
func WithChannel(channel channels.Channel) EngineOption {
	return func(e *Engine) {
		e.channel = channel
	}
}

// defaultBatchProcessingWorkers number of concurrent workers that process incoming blocks.
const defaultBatchProcessingWorkers = 4

// defaultBlockQueueCapacity maximum capacity of inbound queue for batches of BlocksBatch.
const defaultBlockQueueCapacity = 100

// defaultPendingConnectedBlocksChanCapacity capacity of buffered channel that is used to receive pending blocks that form a sequence.
const defaultPendingConnectedBlocksChanCapacity = 100

// Engine is the highest level structure that consumes events from other components.
// It's an entry point to the follower engine which follows and maintains the local copy of the protocol state.
// It is a passive (read-only) version of the compliance engine. The compliance engine
// is employed by consensus nodes (active consensus participants) where the
// Follower engine is employed by all other node roles.
// Engine is responsible for:
// 1. Consuming events from external sources such as sync engine.
// 2. Splitting incoming batches in batches of connected blocks.
// 3. Providing worker goroutines for concurrent processing of batches of connected blocks.
// 4. Handling of finalization events.
// Implements consensus.Compliance interface.
type Engine struct {
	*component.ComponentManager
	log                        zerolog.Logger
	me                         module.Local
	engMetrics                 module.EngineMetrics
	con                        network.Conduit
	channel                    channels.Channel
	headers                    storage.Headers
	pendingBlocks              *fifoqueue.FifoQueue        // queue for processing inbound batches of blocks
	pendingBlocksNotifier      engine.Notifier             // notifies that new batches are ready to be processed
	finalizedBlockTracker      *tracker.NewestBlockTracker // tracks the latest finalization block
	finalizedBlockNotifier     engine.Notifier             // notifies when the latest finalized block changes
	pendingConnectedBlocksChan chan flow.Slashable[[]*flow.Block]
	core                       common.FollowerCore // performs actual processing of incoming messages.
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ consensus.Compliance = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	engMetrics module.EngineMetrics,
	headers storage.Headers,
	finalized *flow.Header,
	core common.FollowerCore,
	opts ...EngineOption,
) (*Engine, error) {
	// FIFO queue for block proposals
	pendingBlocks, err := fifoqueue.NewFifoQueue(defaultBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound blocks: %w", err)
	}

	e := &Engine{
		log:                        log.With().Str("engine", "follower").Logger(),
		me:                         me,
		engMetrics:                 engMetrics,
		channel:                    channels.ReceiveBlocks,
		pendingBlocks:              pendingBlocks,
		pendingBlocksNotifier:      engine.NewNotifier(),
		pendingConnectedBlocksChan: make(chan flow.Slashable[[]*flow.Block], defaultPendingConnectedBlocksChanCapacity),
		finalizedBlockTracker:      tracker.NewNewestBlockTracker(),
		finalizedBlockNotifier:     engine.NewNotifier(),
		headers:                    headers,
		core:                       core,
	}
	e.finalizedBlockTracker.Track(model.BlockFromFlow(finalized))

	for _, apply := range opts {
		apply(e)
	}

	con, err := net.Register(e.channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine to network: %w", err)
	}
	e.con = con

	cmBuilder := component.NewComponentManagerBuilder().
		AddWorker(e.finalizationProcessingLoop).
		AddWorker(e.processBlocksLoop)

	cmBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// start internal component
		e.core.Start(ctx)
		// wait for it to be ready
		<-e.core.Ready()

		// report that we are ready to process events
		ready()

		// wait for shutdown to be commenced
		<-ctx.Done()
		// wait for core to shut down
		<-e.core.Done()
	})

	for i := 0; i < defaultBatchProcessingWorkers; i++ {
		cmBuilder.AddWorker(e.processConnectedBatch)
	}
	e.ComponentManager = cmBuilder.Build()

	return e, nil
}

// OnBlockProposal logs an error and drops the proposal. This is because the follower ingests new
// blocks directly from the networking layer (channel `channels.ReceiveBlocks` by default), which
// delivers its messages by calling the generic `Process` method. Receiving block proposal as
// from another internal component is likely an implementation bug.
func (e *Engine) OnBlockProposal(_ flow.Slashable[*messages.BlockProposal]) {
	e.log.Error().Msg("received unexpected block proposal via internal method")
}

// OnSyncedBlocks consumes incoming blocks by pushing into queue and notifying worker.
func (e *Engine) OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal]) {
	e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageSyncedBlocks)
	// The synchronization engine feeds the follower with batches of blocks. The field `Slashable.OriginID`
	// states which node forwarded the batch to us. Each block contains its proposer and signature.

	if e.pendingBlocks.Push(blocks) {
		e.pendingBlocksNotifier.Notify()
	}
}

// OnFinalizedBlock implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// It informs follower.Core about finalization of the respective block.
//
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedBlockTracker.Track(block) {
		e.finalizedBlockNotifier.Notify()
	}
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *messages.BlockProposal:
		e.onBlockProposal(flow.Slashable[*messages.BlockProposal]{
			OriginID: originID,
			Message:  msg,
		})
	default:
		e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
	}
	return nil
}

// processBlocksLoop processes available blocks as they are queued.
func (e *Engine) processBlocksLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newPendingBlockSignal := e.pendingBlocksNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-newPendingBlockSignal:
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
			if len(batch.Message) < 1 {
				continue
			}
			blocks := make([]*flow.Block, 0, len(batch.Message))
			for _, block := range batch.Message {
				blocks = append(blocks, block.Block.ToInternal())
			}

			firstBlock := blocks[0].Header
			lastBlock := blocks[len(blocks)-1].Header
			log := e.log.With().
				Hex("origin_id", batch.OriginID[:]).
				Str("chain_id", lastBlock.ChainID.String()).
				Uint64("first_block_height", firstBlock.Height).
				Uint64("first_block_view", firstBlock.View).
				Uint64("last_block_height", lastBlock.Height).
				Uint64("last_block_view", lastBlock.View).
				Int("range_length", len(blocks)).
				Logger()

			latestFinalizedView := e.finalizedBlockTracker.NewestBlock().View
			submitConnectedBatch := func(blocks []*flow.Block) {
				e.submitConnectedBatch(log, latestFinalizedView, batch.OriginID, blocks)
			}

			// extract sequences of connected blocks and schedule them for further processing
			// we assume the sender has already ordered blocks into connected ranges if possible
			parentID := blocks[0].ID()
			indexOfLastConnected := 0
			for i := 1; i < len(blocks); i++ {
				if blocks[i].Header.ParentID != parentID {
					submitConnectedBatch(blocks[indexOfLastConnected:i])
					indexOfLastConnected = i
				}
				parentID = blocks[i].Header.ID()
			}
			submitConnectedBatch(blocks[indexOfLastConnected:])

			e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// submitConnectedBatch checks if batch is still pending and submits it via channel for further processing by worker goroutines.
func (e *Engine) submitConnectedBatch(log zerolog.Logger, latestFinalizedView uint64, originID flow.Identifier, blocks []*flow.Block) {
	if len(blocks) < 1 {
		return
	}
	// if latest block of batch is already finalized we can drop such input.
	lastBlock := blocks[len(blocks)-1].Header
	if lastBlock.View < latestFinalizedView {
		log.Debug().Msgf("dropping range [%d, %d] below finalized view %d", blocks[0].Header.View, lastBlock.View, latestFinalizedView)
		return
	}
	log.Debug().Msgf("submitting sub-range with views [%d, %d] for further processing", blocks[0].Header.View, lastBlock.View)

	select {
	case e.pendingConnectedBlocksChan <- flow.Slashable[[]*flow.Block]{
		OriginID: originID,
		Message:  blocks,
	}:
	case <-e.ComponentManager.ShutdownSignal():
	}
}

// processConnectedBatch is a worker goroutine which concurrently consumes connected batches that will be processed by Core.
func (e *Engine) processConnectedBatch(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-e.pendingConnectedBlocksChan:
			err := e.core.OnBlockRange(msg.OriginID, msg.Message)
			if err != nil {
				ctx.Throw(err)
			}
		}
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
			e.core.OnFinalizedBlock(finalHeader)
		}
	}
}

// onBlockProposal performs processing of incoming block by pushing into queue and notifying worker.
func (e *Engine) onBlockProposal(proposal flow.Slashable[*messages.BlockProposal]) {
	e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageBlockProposal)
	proposalAsList := flow.Slashable[[]*messages.BlockProposal]{
		OriginID: proposal.OriginID,
		Message:  []*messages.BlockProposal{proposal.Message},
	}
	// queue proposal
	if e.pendingBlocks.Push(proposalAsList) {
		e.pendingBlocksNotifier.Notify()
	}
}
