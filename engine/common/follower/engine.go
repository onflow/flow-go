package follower

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
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
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

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
	config                compliance.Config
	me                    module.Local
	engMetrics            module.EngineMetrics
	mempoolMetrics        module.MempoolMetrics
	cleaner               storage.Cleaner
	headers               storage.Headers
	payloads              storage.Payloads
	state                 protocol.FollowerState
	pending               module.PendingBlockBuffer
	follower              module.HotStuffFollower
	validator             hotstuff.Validator
	con                   network.Conduit
	sync                  module.BlockRequester
	tracer                module.Tracer
	channel               channels.Channel
	pendingBlocks         *fifoqueue.FifoQueue // queues for processing inbound blocks
	pendingBlocksNotifier engine.Notifier
}

type Option func(*Engine)

// WithComplianceOptions sets options for the engine's compliance config
func WithComplianceOptions(opts ...compliance.Opt) Option {
	return func(e *Engine) {
		for _, apply := range opts {
			apply(&e.config)
		}
	}
}

// WithChannel sets the channel the follower engine will use to receive blocks.
func WithChannel(channel channels.Channel) Option {
	return func(e *Engine) {
		e.channel = channel
	}
}

var _ network.MessageProcessor = (*Engine)(nil)
var _ consensus.Compliance = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.FollowerState,
	pending module.PendingBlockBuffer,
	follower module.HotStuffFollower,
	validator hotstuff.Validator,
	sync module.BlockRequester,
	tracer module.Tracer,
	opts ...Option,
) (*Engine, error) {
	// FIFO queue for block proposals
	pendingBlocks, err := fifoqueue.NewFifoQueue(defaultBlockQueueCapacity)
	if err != nil {
		return nil, fmt.Errorf("failed to create queue for inbound blocks: %w", err)
	}

	e := &Engine{
		log:                   log.With().Str("engine", "follower").Logger(),
		config:                compliance.DefaultConfig(),
		me:                    me,
		engMetrics:            engMetrics,
		mempoolMetrics:        mempoolMetrics,
		cleaner:               cleaner,
		headers:               headers,
		payloads:              payloads,
		state:                 state,
		pending:               pending,
		follower:              follower,
		validator:             validator,
		sync:                  sync,
		tracer:                tracer,
		channel:               channels.ReceiveBlocks,
		pendingBlocks:         pendingBlocks,
		pendingBlocksNotifier: engine.NewNotifier(),
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
func (e *Engine) OnBlockProposal(_ flow.Slashable[*messages.BlockProposal]) {
	e.log.Error().Msg("received unexpected block proposal via internal method")
}

// OnSyncedBlocks performs processing of incoming blocks by pushing into queue and notifying worker.
func (e *Engine) OnSyncedBlocks(blocks flow.Slashable[[]*messages.BlockProposal]) {
	e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageSyncedBlocks)
	// a blocks batch that is synced has to come locally, from the synchronization engine
	// the block itself will contain the proposer to indicate who created it

	// queue proposal
	if e.pendingBlocks.Push(blocks) {
		e.pendingBlocksNotifier.Notify()
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
				err := e.processBlockProposal(batch.OriginID, block)
				if err != nil {
					return fmt.Errorf("could not handle block proposal: %w", err)
				}
				e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
			}
			continue
		}

		// when there are no more messages in the queue, back to the processBlocksLoop to wait
		// for the next incoming message to arrive.
		return nil
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

// processBlockProposal handles incoming block proposals.
// No errors are expected during normal operations.
func (e *Engine) processBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {
	block := proposal.Block.ToInternal()
	header := block.Header
	blockID := header.ID()

	span, ctx := e.tracer.StartBlockSpan(context.Background(), blockID, trace.FollowerOnBlockProposal)
	defer span.End()

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Logger()

	log.Info().Msg("block proposal received")

	e.prunePendingCache()

	// first, we reject all blocks that we don't need to process:
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization
	// 3) blocks at a height below finalized height; they can not be finalized

	// ignore proposals that are already cached
	_, cached := e.pending.ByID(blockID)
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := e.headers.ByBlockID(blockID)
	if err == nil {
		log.Debug().Msg("skipping already processed proposal")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check proposal: %w", err)
	}

	// ignore proposals which are too far ahead of our local finalized state
	// instead, rely on sync engine to catch up finalization more effectively, and avoid
	// large subtree of blocks to be cached.
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get latest finalized header: %w", err)
	}
	if header.Height > final.Height && header.Height-final.Height > e.config.SkipNewProposalsThreshold {
		log.Debug().
			Uint64("final_height", final.Height).
			Msg("dropping block too far ahead of locally finalized height")
		return nil
	}
	if header.Height <= final.Height {
		log.Debug().
			Uint64("final_height", final.Height).
			Msg("dropping block below finalized threshold")
		return nil
	}

	// there are two possibilities if the proposal is neither already pending
	// processing in the cache, nor has already been processed:
	// 1) the proposal is unverifiable because parent or ancestor is unknown
	// => we cache the proposal and request the missing link
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if the parent is a pending block (disconnected from the incorporated state), we cache this block as well.
	// we don't have to request its parent block or its ancestor again, because as a
	// pending block, its parent block must have been requested.
	// if there was problem requesting its parent or ancestors, the sync engine's forward
	// syncing with range requests for finalized blocks will request for the blocks.
	_, found := e.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = e.pending.Add(originID, block)
		e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = e.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		_ = e.pending.Add(originID, block)

		log.Debug().Msg("requesting missing parent for proposal")

		e.sync.RequestBlock(header.ParentID, header.Height-1)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	// at this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not
	err = e.processBlockAndDescendants(ctx, block)
	if err != nil {
		return fmt.Errorf("could not process block proposal (id=%x, height=%d, view=%d): %w", blockID, header.Height, header.View, err)
	}

	// most of the heavy database checks are done at this point, so this is a
	// good moment to potentially kick-off a garbage collection of the DB
	// NOTE: this is only effectively run every 1000th calls, which corresponds
	// to every 1000th successfully processed block
	e.cleaner.RunGC()

	return nil
}

// processBlockAndDescendants processes `proposal` and its pending descendants recursively.
// The function assumes that `proposal` is connected to the finalized state. By induction,
// any children are therefore also connected to the finalized state and can be processed as well.
// No errors are expected during normal operations.
func (e *Engine) processBlockAndDescendants(ctx context.Context, proposal *flow.Block) error {
	header := proposal.Header
	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.FollowerProcessBlockProposal)
	defer span.End()

	log := e.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Logger()

	log.Info().Msg("processing block proposal")

	hotstuffProposal := model.ProposalFromFlow(header)
	err := e.validator.ValidateProposal(hotstuffProposal)
	if err != nil {
		if model.IsInvalidBlockError(err) {
			// TODO potential slashing
			log.Err(err).Msgf("received invalid block proposal (potential slashing evidence)")
			return nil
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
			log.Err(err).Msg("unable to validate proposal with view from unknown epoch")
			return nil
		}
		return fmt.Errorf("unexpected error validating proposal: %w", err)
	}

	// check whether the block is a valid extension of the chain.
	// The follower engine only checks the block's header. The more expensive payload validation
	// is only done by the consensus committee. For safety, we require that a QC for the extending
	// block is provided while inserting the block. This ensures that all stored blocks are fully validated
	// by the consensus committee before being stored here.
	err = e.state.ExtendCertified(ctx, proposal, nil)
	if err != nil {
		// block is outdated by the time we started processing it
		// => some other node generating the proposal is probably behind is catching up.
		if state.IsOutdatedExtensionError(err) {
			log.Info().Err(err).Msg("dropped processing of abandoned fork; this might be an indicator that some consensus node is behind")
			return nil
		}
		// the block is invalid; log as error as we desire honest participation
		// ToDo: potential slashing
		if state.IsInvalidExtensionError(err) {
			log.Warn().
				Err(err).
				Msg("received invalid block from other node (potential slashing evidence?)")
			return nil
		}

		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	log.Info().Msg("forwarding block proposal to hotstuff")

	// submit the model to follower for processing
	e.follower.SubmitProposal(hotstuffProposal)

	// check for any descendants of the block to process
	err = e.processPendingChildren(ctx, header)
	if err != nil {
		return fmt.Errorf("could not process pending children: %w", err)
	}

	return nil
}

// processPendingChildren checks if there are proposals connected to the given
// parent block that was just processed; if this is the case, they should now
// all be validly connected to the finalized state and we should process them.
func (e *Engine) processPendingChildren(ctx context.Context, header *flow.Header) error {

	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.FollowerProcessPendingChildren)
	defer span.End()

	blockID := header.ID()

	// check if there are any children for this parent in the cache
	children, has := e.pending.ByParentID(blockID)
	if !has {
		return nil
	}

	// then try to process children only this once
	var result *multierror.Error
	for _, child := range children {
		err := e.processBlockAndDescendants(ctx, child.Message)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// drop all the children that should have been processed now
	e.pending.DropForParent(blockID)

	return result.ErrorOrNil()
}

// prunePendingCache prunes the pending block cache.
func (e *Engine) prunePendingCache() {

	// retrieve the finalized height
	final, err := e.state.Final().Head()
	if err != nil {
		e.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
		return
	}

	// remove all pending blocks at or below the finalized view
	e.pending.PruneByView(final.View)

	// always record the metric
	e.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, e.pending.Size())
}
