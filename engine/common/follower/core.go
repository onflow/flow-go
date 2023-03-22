package follower

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/engine/common"
	"github.com/onflow/flow-go/engine/common/follower/cache"
	"github.com/onflow/flow-go/engine/common/follower/pending_tree"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type ComplianceOption func(*Core)

// WithComplianceOptions sets options for the core's compliance config
func WithComplianceOptions(opts ...compliance.Opt) ComplianceOption {
	return func(c *Core) {
		for _, apply := range opts {
			apply(&c.config)
		}
	}
}

// Core implements main processing logic for follower engine.
// Generally is NOT concurrency safe but some functions can be used in concurrent setup.
type Core struct {
	*component.ComponentManager
	log                 zerolog.Logger
	mempoolMetrics      module.MempoolMetrics
	config              compliance.Config
	tracer              module.Tracer
	pendingCache        *cache.Cache
	pendingTree         *pending_tree.PendingTree
	cleaner             storage.Cleaner
	state               protocol.FollowerState
	follower            module.HotStuffFollower
	validator           hotstuff.Validator
	sync                module.BlockRequester
	certifiedBlocksChan chan CertifiedBlocks // delivers batches of certified blocks to main core worker
	finalizedBlocksChan chan *flow.Header    // delivers finalized blocks to main core worker.
}

var _ common.FollowerCore = (*Core)(nil)

func NewCore(log zerolog.Logger,
	mempoolMetrics module.MempoolMetrics,
	cleaner storage.Cleaner,
	payloads storage.Payloads,
	state protocol.FollowerState,
	pending module.PendingBlockBuffer,
	follower module.HotStuffFollower,
	validator hotstuff.Validator,
	sync module.BlockRequester,
	tracer module.Tracer,
	opts ...ComplianceOption) *Core {
	metricsCollector := metrics.NewNoopCollector()
	onEquivocation := func(block, otherBlock *flow.Block) {}
	c := &Core{
		log:                 log.With().Str("engine", "follower_core").Logger(),
		mempoolMetrics:      mempoolMetrics,
		cleaner:             cleaner,
		state:               state,
		pendingCache:        cache.NewCache(log, 1000, metricsCollector, onEquivocation),
		follower:            follower,
		validator:           validator,
		sync:                sync,
		tracer:              tracer,
		config:              compliance.DefaultConfig(),
		certifiedBlocksChan: make(chan CertifiedBlocks, defaultCertifiedBlocksChannelCapacity),
		finalizedBlocksChan: make(chan *flow.Header, 10),
	}

	for _, apply := range opts {
		apply(c)
	}

	c.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(c.processCoreSeqEvents).
		Build()

	return c
}

// OnBlockProposal handles incoming block proposals.
// No errors are expected during normal operations.
//func (c *Core) OnBlockProposal(originID flow.Identifier, batch []*messages.BlockProposal) error {
//	block := proposal.Block.ToInternal()
//	header := block.Header
//	blockID := header.ID()
//
//	span, ctx := c.tracer.StartBlockSpan(context.Background(), blockID, trace.FollowerOnBlockProposal)
//	defer span.End()
//
//	log := c.log.With().
//		Hex("origin_id", originID[:]).
//		Str("chain_id", header.ChainID.String()).
//		Uint64("block_height", header.Height).
//		Uint64("block_view", header.View).
//		Hex("block_id", blockID[:]).
//		Hex("parent_id", header.ParentID[:]).
//		Hex("payload_hash", header.PayloadHash[:]).
//		Time("timestamp", header.Timestamp).
//		Hex("proposer", header.ProposerID[:]).
//		Logger()
//
//	log.Info().Msg("block proposal received")
//
//	// first, we reject all blocks that we don't need to process:
//	// 1) blocks already in the cache; they will already be processed later
//	// 2) blocks already on disk; they were processed and await finalization
//	// 3) blocks at a height below finalized height; they can not be finalized
//
//	// ignore proposals that are already cached
//	_, cached := c.pendingCache.ByID(blockID)
//	if cached {
//		log.Debug().Msg("skipping already cached proposal")
//		return nil
//	}
//
//	// ignore proposals that were already processed
//	_, err := c.headers.ByBlockID(blockID)
//	if err == nil {
//		log.Debug().Msg("skipping already processed proposal")
//		return nil
//	}
//	if !errors.Is(err, storage.ErrNotFound) {
//		return fmt.Errorf("could not check proposal: %w", err)
//	}
//
//	// ignore proposals which are too far ahead of our local finalized state
//	// instead, rely on sync engine to catch up finalization more effectively, and avoid
//	// large subtree of blocks to be cached.
//	final, err := c.state.Final().Head()
//	if err != nil {
//		return fmt.Errorf("could not get latest finalized header: %w", err)
//	}
//	if header.Height > final.Height && header.Height-final.Height > c.config.SkipNewProposalsThreshold {
//		log.Debug().
//			Uint64("final_height", final.Height).
//			Msg("dropping block too far ahead of locally finalized height")
//		return nil
//	}
//	if header.Height <= final.Height {
//		log.Debug().
//			Uint64("final_height", final.Height).
//			Msg("dropping block below finalized threshold")
//		return nil
//	}
//
//	// there are two possibilities if the proposal is neither already pendingCache
//	// processing in the cache, nor has already been processed:
//	// 1) the proposal is unverifiable because parent or ancestor is unknown
//	// => we cache the proposal and request the missing link
//	// 2) the proposal is connected to finalized state through an unbroken chain
//	// => we verify the proposal and forward it to hotstuff if valid
//
//	// if the parent is a pendingCache block (disconnected from the incorporated state), we cache this block as well.
//	// we don't have to request its parent block or its ancestor again, because as a
//	// pendingCache block, its parent block must have been requested.
//	// if there was problem requesting its parent or ancestors, the sync engine's forward
//	// syncing with range requests for finalized blocks will request for the blocks.
//	_, found := c.pendingCache.ByID(header.ParentID)
//	if found {
//
//		// add the block to the cache
//		_ = c.pendingCache.Add(originID, block)
//		c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pendingCache.Size())
//
//		return nil
//	}
//
//	// if the proposal is connected to a block that is neither in the cache, nor
//	// in persistent storage, its direct parent is missing; cache the proposal
//	// and request the parent
//	_, err = c.headers.ByBlockID(header.ParentID)
//	if errors.Is(err, storage.ErrNotFound) {
//
//		_ = c.pendingCache.Add(originID, block)
//
//		log.Debug().Msg("requesting missing parent for proposal")
//
//		c.sync.RequestBlock(header.ParentID, header.Height-1)
//
//		return nil
//	}
//	if err != nil {
//		return fmt.Errorf("could not check parent: %w", err)
//	}
//
//	// at this point, we should be able to connect the proposal to the finalized
//	// state and should process it to see whether to forward to hotstuff or not
//	err = c.processBlockAndDescendants(ctx, block)
//	if err != nil {
//		return fmt.Errorf("could not process block proposal (id=%x, height=%d, view=%d): %w", blockID, header.Height, header.View, err)
//	}
//
//	// most of the heavy database checks are done at this point, so this is a
//	// good moment to potentially kick-off a garbage collection of the DB
//	// NOTE: this is only effectively run every 1000th calls, which corresponds
//	// to every 1000th successfully processed block
//	c.cleaner.RunGC()
//
//	return nil
//}

func (c *Core) OnBlockRange(originID flow.Identifier, batch []*flow.Block) error {
	certifiedBatch, certifyingQC, err := c.pendingCache.AddBlocks(batch)
	if err != nil {
		return fmt.Errorf("could not add batch of pendingCache blocks: %w", err)
	}
	certifiedBlocks := make(CertifiedBlocks, 0, len(certifiedBatch))
	for i := 0; i < len(certifiedBatch); i++ {
		block := certifiedBatch[i]
		var qc *flow.QuorumCertificate
		if i < len(certifiedBatch)-1 {
			qc = certifiedBatch[i+1].Header.QuorumCertificate()
		} else {
			qc = certifyingQC
		}
		certifiedBlocks = append(certifiedBlocks, pending_tree.CertifiedBlock{
			Block: block,
			QC:    qc,
		})
	}

	c.certifiedBlocksChan <- certifiedBlocks

	return nil
}

// processCoreSeqEvents processes events that need to be dispatched on dedicated core's goroutine.
// Here we process events that need to be sequentially ordered(processing certified blocks and new finalized blocks).
func (c *Core) processCoreSeqEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	doneSignal := ctx.Done()
	for {
		select {
		case <-doneSignal:
			return
		case finalized := <-c.finalizedBlocksChan:
			err := c.processFinalizedBlock(finalized) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		case blocks := <-c.certifiedBlocksChan:
			err := c.processCertifiedBlocks(blocks) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// processBlockAndDescendants processes `proposal` and its pendingCache descendants recursively.
// The function assumes that `proposal` is connected to the finalized state. By induction,
// any children are therefore also connected to the finalized state and can be processed as well.
// No errors are expected during normal operations.
func (c *Core) processBlockAndDescendants(ctx context.Context, proposal *flow.Block) error {
	header := proposal.Header
	span, ctx := c.tracer.StartSpanFromContext(ctx, trace.FollowerProcessBlockProposal)
	defer span.End()

	log := c.log.With().
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
	err := c.validator.ValidateProposal(hotstuffProposal)
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
	err = c.state.ExtendCertified(ctx, proposal, nil)
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

	return nil
}

// OnFinalizedBlock updates local state of pendingCache tree using received finalized block.
// Is NOT concurrency safe, has to be used by the same goroutine as processCertifiedBlocks.
// OnFinalizedBlock and processCertifiedBlocks MUST be sequentially ordered.
func (c *Core) OnFinalizedBlock(final *flow.Header) {
	c.pendingCache.PruneUpToView(final.View)
	c.finalizedBlocksChan <- final
}

// processCertifiedBlocks processes batch of certified blocks by applying them to tree of certified blocks.
// As result of this operation we might extend protocol state.
// Is NOT concurrency safe, has to be used by the same goroutine as OnFinalizedBlock.
// OnFinalizedBlock and processCertifiedBlocks MUST be sequentially ordered.
func (c *Core) processCertifiedBlocks(blocks CertifiedBlocks) error {
	for _, certifiedBlock := range blocks {
		err := c.state.ExtendCertified(context.Background(), certifiedBlock.Block, certifiedBlock.QC)
		if err != nil {
			if state.IsOutdatedExtensionError(err) {
				continue
			}
			return fmt.Errorf("could not extend protocol state with certified block: %w", err)
		}

		hotstuffProposal := model.ProposalFromFlow(certifiedBlock.Block.Header)

		// submit the model to follower for processing
		c.follower.SubmitProposal(hotstuffProposal)
	}
	return nil
}

func (c *Core) processFinalizedBlock(finalized *flow.Header) error {
	panic("implement me")
}
