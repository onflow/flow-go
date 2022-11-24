package follower

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type Engine struct {
	unit           *engine.Unit
	log            zerolog.Logger
	config         compliance.Config
	me             module.Local
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	cleaner        storage.Cleaner
	headers        storage.Headers
	payloads       storage.Payloads
	state          protocol.MutableState
	pending        module.PendingBlockBuffer
	follower       module.HotStuffFollower
	con            network.Conduit
	sync           module.BlockRequester
	tracer         module.Tracer
	channel        channels.Channel
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

func New(
	log zerolog.Logger,
	net network.Network,
	me module.Local,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.MutableState,
	pending module.PendingBlockBuffer,
	follower module.HotStuffFollower,
	sync module.BlockRequester,
	tracer module.Tracer,
	opts ...Option,
) (*Engine, error) {
	e := &Engine{
		unit:           engine.NewUnit(),
		log:            log.With().Str("engine", "follower").Logger(),
		config:         compliance.DefaultConfig(),
		me:             me,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		cleaner:        cleaner,
		headers:        headers,
		payloads:       payloads,
		state:          state,
		pending:        pending,
		follower:       follower,
		sync:           sync,
		tracer:         tracer,
		channel:        channels.ReceiveBlocks,
	}

	for _, apply := range opts {
		apply(e)
	}

	con, err := net.Register(e.channel, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine to network: %w", err)
	}
	e.con = con

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		<-e.follower.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.follower.Done()
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel channels.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, input interface{}) error {
	switch v := input.(type) {
	case *messages.BlockResponse:
		e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageBlockResponse)
		defer e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockResponse)
		e.unit.Lock()
		defer e.unit.Unlock()
		return e.onBlockResponse(originID, v)
	case *events.SyncedBlock:
		e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageSyncedBlock)
		defer e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageSyncedBlock)
		e.unit.Lock()
		defer e.unit.Unlock()
		return e.onSyncedBlock(originID, v)
	case *messages.BlockProposal:
		e.engMetrics.MessageReceived(metrics.EngineFollower, metrics.MessageBlockProposal)
		defer e.engMetrics.MessageHandled(metrics.EngineFollower, metrics.MessageBlockProposal)
		e.unit.Lock()
		defer e.unit.Unlock()
		return e.onBlockProposal(originID, v, false)
	default:
		return fmt.Errorf("invalid event type (%T)", input)
	}
}

func (e *Engine) onSyncedBlock(originID flow.Identifier, synced *events.SyncedBlock) error {

	// a block that is synced has to come locally, from the synchronization engine
	// the block itself will contain the proposer to indicate who created it
	if originID != e.me.NodeID() {
		return fmt.Errorf("synced block with non-local origin (local: %x, origin: %x)", e.me.NodeID(), originID)
	}

	// process as proposal
	proposal := &messages.BlockProposal{
		Header:  synced.Block.Header,
		Payload: synced.Block.Payload,
	}
	return e.onBlockProposal(originID, proposal, false)
}

func (e *Engine) onBlockResponse(originID flow.Identifier, res *messages.BlockResponse) error {
	for i, block := range res.Blocks {
		proposal := &messages.BlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		// process block proposal with a wait
		if err := e.onBlockProposal(originID, proposal, true); err != nil {
			return fmt.Errorf("fail to process the block at index %v in a range block response that contains %v blocks: %w", i, len(res.Blocks), err)
		}
	}

	return nil
}

// onBlockProposal handles incoming block proposals. inRangeBlockResponse will determine whether or not we should wait in processBlockAndDescendants
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal, inRangeBlockResponse bool) error {

	span, ctx, _ := e.tracer.StartBlockSpan(context.Background(), proposal.Header.ID(), trace.FollowerOnBlockProposal)
	defer span.End()

	header := proposal.Header

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
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
	_, cached := e.pending.ByID(header.ID())
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := e.headers.ByBlockID(header.ID())
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

	// there are two possibilities if the proposal is neither already pending
	// processing in the cache, nor has already been processed:
	// 1) the proposal is unverifiable because parent or ancestor is unknown
	// => we cache the proposal and request the missing link
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if we can connect the proposal to an ancestor in the cache, it means
	// there is a missing link; we cache it and request the missing link
	ancestor, found := e.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = e.pending.Add(originID, proposal)

		// go to the first missing ancestor
		ancestorID := ancestor.Header.ParentID
		ancestorHeight := ancestor.Header.Height - 1
		for {
			ancestor, found = e.pending.ByID(ancestorID)
			if !found {
				break
			}
			ancestorID = ancestor.Header.ParentID
			ancestorHeight = ancestor.Header.Height - 1
		}

		log.Debug().
			Uint64("ancestor_height", ancestorHeight).
			Hex("ancestor_id", ancestorID[:]).
			Msg("requesting missing ancestor for proposal")

		e.sync.RequestBlock(ancestorID, ancestorHeight)

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = e.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		_ = e.pending.Add(originID, proposal)

		log.Debug().Msg("requesting missing parent for proposal")

		e.sync.RequestBlock(header.ParentID, header.Height-1)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	// at this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not
	err = e.processBlockAndDescendants(ctx, proposal, inRangeBlockResponse)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
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
func (e *Engine) processBlockAndDescendants(ctx context.Context, proposal *messages.BlockProposal, inRangeBlockResponse bool) error {

	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.FollowerProcessBlockProposal)
	defer span.End()

	header := proposal.Header

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

	// see if the block is a valid extension of the protocol state
	block := &flow.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}

	// check whether the block is a valid extension of the chain.
	// it only checks the block header, since checking block body is expensive.
	// The full block check is done by the consensus participants.
	err := e.state.Extend(ctx, block)
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

	// retrieve the parent
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	log.Info().Msg("forwarding block proposal to hotstuff")

	// submit the model to follower for processing
	if inRangeBlockResponse {
		<-e.follower.SubmitProposal(header, parent.View)
	} else {
		// ignore returned channel to avoid waiting
		e.follower.SubmitProposal(header, parent.View)
	}

	// check for any descendants of the block to process
	err = e.processPendingChildren(ctx, header, inRangeBlockResponse)
	if err != nil {
		return fmt.Errorf("could not process pending children: %w", err)
	}

	return nil
}

// processPendingChildren checks if there are proposals connected to the given
// parent block that was just processed; if this is the case, they should now
// all be validly connected to the finalized state and we should process them.
func (e *Engine) processPendingChildren(ctx context.Context, header *flow.Header, inRangeBlockResponse bool) error {

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
		proposal := &messages.BlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := e.processBlockAndDescendants(ctx, proposal, inRangeBlockResponse)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// drop all of the children that should have been processed now
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
