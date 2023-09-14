package follower

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/consensus"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/storage"
)

type EngineOption func(*ComplianceEngine)

// WithChannel sets the channel the follower engine will use to receive blocks.
func WithChannel(channel channels.Channel) EngineOption {
	return func(e *ComplianceEngine) {
		e.channel = channel
	}
}

// WithComplianceConfigOpt applies compliance config opt to internal config
func WithComplianceConfigOpt(opt compliance.Opt) EngineOption {
	return func(e *ComplianceEngine) {
		opt(&e.config)
	}
}

// defaultBatchProcessingWorkers number of concurrent workers that process incoming blocks.
const defaultBatchProcessingWorkers = 4

// defaultPendingBlockQueueCapacity maximum capacity of inbound queue for blocks directly received from other nodes.
// Small capacity is suitable here, as there will be hardly any pending blocks during normal operations. If the node
// is so overloaded that it can't keep up with the newest blocks within 10 seconds (processing them with priority),
// it is probably better to fall back on synchronization anyway.
const defaultPendingBlockQueueCapacity = 10

// defaultSyncedBlockQueueCapacity maximum capacity of inbound queue for batches of synced blocks.
// While catching up, we want to be able to buffer a bit larger amount of work.
const defaultSyncedBlockQueueCapacity = 100

// defaultPendingConnectedBlocksChanCapacity capacity of buffered channel that is used to receive pending blocks that form a sequence.
const defaultPendingConnectedBlocksChanCapacity = 100

// ComplianceEngine is the highest level structure that consumes events from other components.
// It's an entry point to the follower engine which follows and maintains the local copy of the protocol state.
// It is a passive (read-only) version of the compliance engine. The compliance engine
// is employed by consensus nodes (active consensus participants) where the
// Follower engine is employed by all other node roles.
// ComplianceEngine is responsible for:
//  1. Consuming events from external sources such as sync engine.
//  2. Splitting incoming batches in batches of connected blocks.
//  3. Providing worker goroutines for concurrent processing of batches of connected blocks.
//  4. Handling of finalization events.
//
// See interface `complianceCore` (this package) for detailed documentation of the algorithm.
// Implements consensus.Compliance interface.
type ComplianceEngine struct {
	*component.ComponentManager
	log                        zerolog.Logger
	me                         module.Local
	engMetrics                 module.EngineMetrics
	con                        network.Conduit
	config                     compliance.Config
	channel                    channels.Channel
	headers                    storage.Headers
	pendingProposals           *fifoqueue.FifoQueue        // queue for fresh proposals
	syncedBlocks               *fifoqueue.FifoQueue        // queue for processing inbound batches of synced blocks
	blocksAvailableNotifier    engine.Notifier             // notifies that new blocks are ready to be processed
	finalizedBlockTracker      *tracker.NewestBlockTracker // tracks the latest finalization block
	finalizedBlockNotifier     engine.Notifier             // notifies when the latest finalized block changes
	pendingConnectedBlocksChan chan flow.Slashable[[]*flow.Block]
	core                       complianceCore // performs actual processing of incoming messages.
}

var _ network.MessageProcessor = (*ComplianceEngine)(nil)
var _ consensus.Compliance = (*ComplianceEngine)(nil)

// NewComplianceLayer instantiates th compliance layer for the consensus follower. See
// interface `complianceCore` (this package) for detailed documentation of the algorithm.
func NewComplianceLayer(
	log zerolog.Logger,
	net network.EngineRegistry,
	me module.Local,
	engMetrics module.EngineMetrics,
	headers storage.Headers,
	finalized *flow.Header,
	core complianceCore,
	config compliance.Config,
	opts ...EngineOption,
) (*ComplianceEngine, error) {
	// FIFO queue for inbound block proposals
	pendingBlocks, err := fifoqueue.NewFifoQueue(defaultPendingBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound blocks: %w", err)
	}
	// FIFO queue for synced blocks
	syncedBlocks, err := fifoqueue.NewFifoQueue(defaultSyncedBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound blocks: %w", err)
	}

	e := &ComplianceEngine{
		log:                        log.With().Str("engine", "follower").Logger(),
		me:                         me,
		engMetrics:                 engMetrics,
		config:                     config,
		channel:                    channels.ReceiveBlocks,
		pendingProposals:           pendingBlocks,
		syncedBlocks:               syncedBlocks,
		blocksAvailableNotifier:    engine.NewNotifier(),
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

// OnBlockProposal queues *untrusted* proposals for further processing and notifies the Engine's
// internal workers. This method is intended for fresh proposals received directly from leaders.
// It can ingest synced blocks as well, but is less performant compared to method `OnSyncedBlocks`.
func (e *ComplianceEngine) OnBlockProposal(proposal flow.Slashable[*messages.BlockProposal]) {
	e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageBlockProposal)
	// queue proposal
	if e.pendingProposals.Push(proposal) {
		e.blocksAvailableNotifier.Notify()
	}
}

// OnSyncedBlocks is an optimized consumer for *untrusted* synced blocks. It is specifically
// efficient for batches of continuously connected blocks (honest nodes supply finalized blocks
// in suitable sequences where possible). Nevertheless, the method tolerates blocks in arbitrary
// order (less efficient), making it robust against byzantine nodes.
func (e *ComplianceEngine) OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal]) {
	e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageSyncedBlocks)
	// The synchronization engine feeds the follower with batches of blocks. The field `Slashable.OriginID`
	// states which node forwarded the batch to us. Each block contains its proposer and signature.

	if e.syncedBlocks.Push(blocks) {
		e.blocksAvailableNotifier.Notify()
	}
}

// OnFinalizedBlock informs the compliance layer about finalization of a new block. It does not block
// and asynchronously executes the internal pruning logic. We accept inputs out of order, and only act
// on inputs with strictly monotonously increasing views.
//
// Implements the `OnFinalizedBlock` callback from the `hotstuff.FinalizationConsumer`
// CAUTION: the input to this callback is treated as trusted; precautions should be taken that messages
// from external nodes cannot be considered as inputs to this function.
func (e *ComplianceEngine) OnFinalizedBlock(block *model.Block) {
	if e.finalizedBlockTracker.Track(block) {
		e.finalizedBlockNotifier.Notify()
	}
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
// This method is intended to be used as a callback by the networking layer,
// notifying us about fresh proposals directly from the consensus leaders.
func (e *ComplianceEngine) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *messages.BlockProposal:
		e.OnBlockProposal(flow.Slashable[*messages.BlockProposal]{
			OriginID: originID,
			Message:  msg,
		})
	default:
		e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
	}
	return nil
}

// processBlocksLoop processes available blocks as they are queued.
// Implements `component.ComponentWorker` signature.
func (e *ComplianceEngine) processBlocksLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newPendingBlockSignal := e.blocksAvailableNotifier.Channel()
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
// Prioritization: In a nutshell, we prioritize the resilience of the happy path over
// performance gains on the recovery path. Details:
//   - We prioritize new proposals. Thereby, it becomes much harder for a malicious node
//     to overwhelm another node through synchronization messages and drown out new blocks
//     for a node that is up-to-date.
//   - On the flip side, new proposals are relatively infrequent compared to the load that
//     synchronization produces for a node that is catching up. In other words, prioritizing
//     the few new proposals first is probably not going to be much of a distraction.
//     Proposals too far in the future are dropped (see parameter `SkipNewProposalsThreshold`
//     in `compliance.Config`), to prevent memory overflow.
//
// No errors are expected during normal operation. All returned exceptions are potential
// symptoms of internal state corruption and should be fatal.
func (e *ComplianceEngine) processQueuedBlocks(doneSignal <-chan struct{}) error {
	for {
		select {
		case <-doneSignal:
			return nil
		default:
		}

		// Priority 1: ingest fresh proposals
		msg, ok := e.pendingProposals.Pop()
		if ok {
			blockMsg := msg.(flow.Slashable[*messages.BlockProposal])
			block := blockMsg.Message.Block.ToInternal()
			log := e.log.With().
				Hex("origin_id", blockMsg.OriginID[:]).
				Str("chain_id", block.Header.ChainID.String()).
				Uint64("view", block.Header.View).
				Uint64("height", block.Header.Height).
				Logger()
			latestFinalizedView := e.finalizedBlockTracker.NewestBlock().View
			e.submitConnectedBatch(log, latestFinalizedView, blockMsg.OriginID, []*flow.Block{block})
			e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
			continue
		}

		// Priority 2: ingest synced blocks
		msg, ok = e.syncedBlocks.Pop()
		if !ok {
			// when there are no more messages in the queue, back to the processQueuedBlocks to wait
			// for the next incoming message to arrive.
			return nil
		}

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

		// extract sequences of connected blocks and schedule them for further processing
		// we assume the sender has already ordered blocks into connected ranges if possible
		latestFinalizedView := e.finalizedBlockTracker.NewestBlock().View
		parentID := blocks[0].ID()
		indexOfLastConnected := 0
		for i, block := range blocks {
			if block.Header.ParentID != parentID {
				e.submitConnectedBatch(log, latestFinalizedView, batch.OriginID, blocks[indexOfLastConnected:i])
				indexOfLastConnected = i
			}
			parentID = block.Header.ID()
		}
		e.submitConnectedBatch(log, latestFinalizedView, batch.OriginID, blocks[indexOfLastConnected:])
		e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageSyncedBlocks)
	}
}

// submitConnectedBatch checks if batch is still pending and submits it via channel for further processing by worker goroutines.
func (e *ComplianceEngine) submitConnectedBatch(log zerolog.Logger, latestFinalizedView uint64, originID flow.Identifier, blocks []*flow.Block) {
	if len(blocks) < 1 {
		return
	}
	// if latest block of batch is already finalized we can drop such input.
	lastBlock := blocks[len(blocks)-1].Header
	if lastBlock.View < latestFinalizedView {
		log.Debug().Msgf("dropping range [%d, %d] below finalized view %d", blocks[0].Header.View, lastBlock.View, latestFinalizedView)
		return
	}
	skipNewProposalsThreshold := e.config.GetSkipNewProposalsThreshold()
	if lastBlock.View > latestFinalizedView+skipNewProposalsThreshold {
		log.Debug().
			Uint64("skip_new_proposals_threshold", skipNewProposalsThreshold).
			Msgf("dropping range [%d, %d] too far ahead of locally finalized view %d",
				blocks[0].Header.View, lastBlock.View, latestFinalizedView)
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

// processConnectedBatch is a worker goroutine which concurrently consumes connected batches that will be processed by ComplianceCore.
func (e *ComplianceEngine) processConnectedBatch(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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

// finalizationProcessingLoop is a separate goroutine that performs processing of finalization events.
// Implements `component.ComponentWorker` signature.
func (e *ComplianceEngine) finalizationProcessingLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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
