package follower

import (
	"fmt"

	"github.com/rs/zerolog"

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
)

type EngineOption func(*Engine)

// WithChannel sets the channel the follower engine will use to receive blocks.
func WithChannel(channel channels.Channel) EngineOption {
	return func(e *Engine) {
		e.channel = channel
	}
}

// defaultBlockQueueCapacity maximum capacity of inbound queue for `messages.BlockProposal`s
const defaultBlockQueueCapacity = 10_000

// Engine follows and maintains the local copy of the protocol state. It is a
// passive (read-only) version of the compliance engine. The compliance engine
// is employed by consensus nodes (active consensus participants) where the
// Follower engine is employed by all other node roles.
// Implements consensus.Compliance interface.
type Engine struct {
	*component.ComponentManager
	log                   zerolog.Logger
	me                    module.Local
	engMetrics            module.EngineMetrics
	con                   network.Conduit
	channel               channels.Channel
	pendingBlocks         *fifoqueue.FifoQueue // queues for processing inbound blocks
	pendingBlocksNotifier engine.Notifier

	core common.FollowerCore
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

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(e.processBlocksLoop).
		Build()

	return e, nil
}

// OnBlockProposal errors when called since follower engine doesn't support direct ingestion via internal method.
func (c *Engine) OnBlockProposal(_ flow.Slashable[*messages.BlockProposal]) {
	c.log.Error().Msg("received unexpected block proposal via internal method")
}

// OnSyncedBlocks performs processing of incoming blocks by pushing into queue and notifying worker.
func (c *Engine) OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal]) {
	c.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageSyncedBlocks)
	// a blocks batch that is synced has to come locally, from the synchronization engine
	// the block itself will contain the proposer to indicate who created it

	// queue proposal
	if c.pendingBlocks.Push(blocks) {
		c.pendingBlocksNotifier.Notify()
	}
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (c *Engine) Process(channel channels.Channel, originID flow.Identifier, message interface{}) error {
	switch msg := message.(type) {
	case *messages.BlockProposal:
		c.onBlockProposal(flow.Slashable[*messages.BlockProposal]{
			OriginID: originID,
			Message:  msg,
		})
	default:
		c.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, message, channel)
	}
	return nil
}

// processBlocksLoop processes available block, vote, and timeout messages as they are queued.
func (c *Engine) processBlocksLoop(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	newMessageSignal := c.pendingBlocksNotifier.Channel()
	for {
		select {
		case <-doneSignal:
			return
		case <-newMessageSignal:
			err := c.processQueuedBlocks(doneSignal) // no errors expected during normal operations
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
func (c *Engine) processQueuedBlocks(doneSignal <-chan struct{}) error {
	for {
		select {
		case <-doneSignal:
			return nil
		default:
		}

		msg, ok := c.pendingBlocks.Pop()
		if ok {
			batch := msg.(flow.Slashable[[]*messages.BlockProposal])
			for _, block := range batch.Message {
				err := c.core.OnBlockProposal(batch.OriginID, block)
				if err != nil {
					return fmt.Errorf("could not handle block proposal: %w", err)
				}
				c.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
			}
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
	}
}

// onBlockProposal performs processing of incoming block by pushing into queue and notifying worker.
func (c *Engine) onBlockProposal(proposal flow.Slashable[*messages.BlockProposal]) {
	c.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageBlockProposal)
	proposalAsList := flow.Slashable[[]*messages.BlockProposal]{
		OriginID: proposal.OriginID,
		Message:  []*messages.BlockProposal{proposal.Message},
	}
	// queue proposal
	if c.pendingBlocks.Push(proposalAsList) {
		c.pendingBlocksNotifier.Notify()
	}
}
