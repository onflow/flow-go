package follower

import (
	"errors"
	"fmt"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/tracker"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/common/fifoqueue"
	"github.com/onflow/flow-go/engine/common/follower/pending_tree"
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

// defaultBlockProcessingWorkers number of concurrent workers that process incoming blocks.
// TODO: update this constant to use multiple workers when Core is ready.
const defaultBlockProcessingWorkers = 1

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.BlockProposal`s
const defaultBlockQueueCapacity = 10_000

// defaultCertifiedBlocksChannelCapacity maximum capacity of buffered channel that is used to transfer
// certified blocks between workers.
const defaultCertifiedBlocksChannelCapacity = 100

type CertifiedBlocks []pending_tree.CertifiedBlock

// Engine is the highest level structure that consumes events from other components.
// It's an entry point to the follower engine which follows and maintains the local copy of the protocol state.
// It is a passive (read-only) version of the compliance engine. The compliance engine
// is employed by consensus nodes (active consensus participants) where the
// Follower engine is employed by all other node roles.
// Engine is responsible for:
// 1. Consuming events from external sources such as sync engine.
// 2. Providing worker goroutines for concurrent processing of incoming blocks.
// 3. Ordering events that is not safe to perform in concurrent environment.
// 4. Handling of finalization events.
// Implements consensus.Compliance interface.
type Engine struct {
	*component.ComponentManager
	log                    zerolog.Logger
	me                     module.Local
	engMetrics             module.EngineMetrics
	con                    network.Conduit
	channel                channels.Channel
	headers                storage.Headers
	pendingBlocks          *fifoqueue.FifoQueue        // queue for processing inbound blocks
	pendingBlocksNotifier  engine.Notifier             // notifies that new blocks are ready to be processed
	finalizedBlockTracker  *tracker.NewestBlockTracker // tracks the latest finalization block
	finalizedBlockNotifier engine.Notifier             // notifies when the latest finalized block changes
	core                   *Core                       // performs actual processing of incoming messages.
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ consensus.Compliance = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	engMetrics module.EngineMetrics,
	core *Core,
	opts ...EngineOption,
) (*Engine, error) {
	// FIFO queue for block proposals
	pendingBlocks, err := fifoqueue.NewFifoQueue(defaultBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound blocks: %w", err)
	}

	e := &Engine{
		log:                   log.With().Str("engine", "follower").Logger(),
		me:                    me,
		engMetrics:            engMetrics,
		channel:               channels.ReceiveBlocks,
		pendingBlocks:         pendingBlocks,
		pendingBlocksNotifier: engine.NewNotifier(),
		core:                  core,
	}

	for _, apply := range opts {
		apply(e)
	}

	con, err := net.Register(e.channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine to network: %w", err)
	}
	e.con = con

	cmBuilder := component.NewComponentManagerBuilder().
		AddWorker(e.finalizationProcessingLoop)

	for i := 0; i < defaultBlockProcessingWorkers; i++ {
		cmBuilder.AddWorker(e.processBlocksLoop)
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

	// queue proposal
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
			blocks, err := e.validateAndFilterBatch(batch)
			if err != nil {
				return fmt.Errorf("could not validate batch: %w", err)
			}

			if len(blocks) < 1 {
				continue
			}

			parentID := blocks[0].ID()
			indexOfLastConnected := 0
			for i := 1; i < len(blocks); i++ {
				if blocks[i].Header.ParentID != parentID {
					err = e.core.OnBlockBatch(batch.OriginID, blocks[indexOfLastConnected:i])
					if err != nil {
						return fmt.Errorf("could not process batch: %w", err)
					}
					indexOfLastConnected = i
				}
			}

			err = e.core.OnBlockBatch(batch.OriginID, blocks[indexOfLastConnected:])
			if err != nil {
				return fmt.Errorf("could not process batch: %w", err)
			}

			e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// validateAndFilterBatch
func (e *Engine) validateAndFilterBatch(msg flow.Slashable[[]*messages.BlockProposal]) ([]*flow.Block, error) {
	latestFinalizedView := e.finalizedBlockTracker.NewestBlock().View
	filtered := make([]*flow.Block, 0, len(msg.Message))

	for _, extBlock := range msg.Message {
		block := extBlock.Block.ToInternal()
		// skip blocks that are already finalized
		if block.Header.View < latestFinalizedView {
			continue
		}

		hotstuffProposal := model.ProposalFromFlow(block.Header)
		// skip block if it's already in cache
		if b := e.core.pendingCache.Peek(hotstuffProposal.Block.BlockID); b != nil {
			continue
		}

		err := e.core.validator.ValidateProposal(hotstuffProposal)
		if err != nil {
			if model.IsInvalidBlockError(err) {
				// TODO potential slashing
				e.log.Err(err).Msgf("received invalid block proposal (potential slashing evidence)")
				continue
			}
			if errors.Is(err, model.ErrViewForUnknownEpoch) {
				// We have received a proposal, but we don't know the epoch its view is within.
				// We know:
				//  - the parent of this block is valid and inserted (ie. we knew the epoch for it)
				//  - if we then see this for the child, one of two things must have happened:
				//    1. the proposer malicious created the block for a view very far in the future (it's invalid)
				//      -> in this case we can disregard the block
				//    2. no blocks have been finalized the epoch commitment deadline, and the epoch end
				//       (breaking a critical assumption - see EpochCommitSafetyThreshold in protocol.Params for details)
				//      -> in this case, the network has encountered a critical failure
				//  - we assume in general that Case 2 will not happen, therefore we can discard this proposal
				e.log.Err(err).Msg("unable to validate proposal with view from unknown epoch")
				continue
			}
			return nil, fmt.Errorf("unexpected error validating proposal: %w", err)
		}
		filtered = append(filtered, block)
	}
	return filtered, nil
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
