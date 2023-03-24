package follower

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/common"
	"github.com/onflow/flow-go/engine/common/follower/cache"
	"github.com/onflow/flow-go/engine/common/follower/pending_tree"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
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

type CertifiedBlocks []pending_tree.CertifiedBlock

// defaultCertifiedBlocksChannelCapacity maximum capacity of buffered channel that is used to transfer
// certified blocks to specific worker.
const defaultCertifiedBlocksChannelCapacity = 100

// defaultFinalizedBlocksChannelCapacity maximum capacity of buffered channel that is used to transfer
// finalized blocks to specific worker.
const defaultFinalizedBlocksChannelCapacity = 10

// defaultPendingBlocksCacheCapacity maximum capacity of cache for pending blocks.
const defaultPendingBlocksCacheCapacity = 1000

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
	state               protocol.FollowerState
	follower            module.HotStuffFollower
	validator           hotstuff.Validator
	sync                module.BlockRequester
	certifiedBlocksChan chan CertifiedBlocks // delivers batches of certified blocks to main core worker
	finalizedBlocksChan chan *flow.Header    // delivers finalized blocks to main core worker.
}

var _ common.FollowerCore = (*Core)(nil)

// NewCore creates new instance of Core.
// No errors expected during normal operations.
func NewCore(log zerolog.Logger,
	mempoolMetrics module.MempoolMetrics,
	heroCacheCollector module.HeroCacheMetrics,
	finalizationConsumer hotstuff.FinalizationConsumer,
	state protocol.FollowerState,
	follower module.HotStuffFollower,
	validator hotstuff.Validator,
	sync module.BlockRequester,
	tracer module.Tracer,
	opts ...ComplianceOption) (*Core, error) {
	onEquivocation := func(block, otherBlock *flow.Block) {
		finalizationConsumer.OnDoubleProposeDetected(model.BlockFromFlow(block.Header), model.BlockFromFlow(otherBlock.Header))
	}

	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not query finalized block: %w", err)
	}

	c := &Core{
		log:                 log.With().Str("engine", "follower_core").Logger(),
		mempoolMetrics:      mempoolMetrics,
		state:               state,
		pendingCache:        cache.NewCache(log, defaultPendingBlocksCacheCapacity, heroCacheCollector, onEquivocation),
		pendingTree:         pending_tree.NewPendingTree(finalizedBlock),
		follower:            follower,
		validator:           validator,
		sync:                sync,
		tracer:              tracer,
		config:              compliance.DefaultConfig(),
		certifiedBlocksChan: make(chan CertifiedBlocks, defaultCertifiedBlocksChannelCapacity),
		finalizedBlocksChan: make(chan *flow.Header, defaultFinalizedBlocksChannelCapacity),
	}

	for _, apply := range opts {
		apply(c)
	}

	// prune cache to latest finalized view
	c.pendingCache.PruneUpToView(finalizedBlock.View)

	c.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(c.processCoreSeqEvents).
		Build()

	return c, nil
}

// OnBlockRange performs processing batches of connected blocks. Input batch has to be sequentially ordered forming a chain.
// Submitting batch with invalid order results in error, such batch will be discarded and exception will be returned.
// Effectively this function validates incoming batch, adds it to cache of pending blocks and possibly schedules blocks for further
// processing if they were certified.
// No errors expected during normal operations.
// This function is safe to use in concurrent environment.
func (c *Core) OnBlockRange(originID flow.Identifier, batch []*flow.Block) error {
	if len(batch) < 1 {
		return nil
	}

	firstBlock := batch[0].Header
	lastBlock := batch[len(batch)-1].Header
	hotstuffProposal := model.ProposalFromFlow(lastBlock)
	log := c.log.With().
		Hex("origin_id", originID[:]).
		Str("chain_id", lastBlock.ChainID.String()).
		Uint64("first_block_height", firstBlock.Height).
		Uint64("first_block_view", firstBlock.View).
		Uint64("last_block_height", lastBlock.Height).
		Uint64("last_block_view", lastBlock.View).
		Hex("last_block_id", hotstuffProposal.Block.BlockID[:]).
		Int("range_length", len(batch)).
		Logger()

	log.Info().Msg("processing block range")

	if c.pendingCache.Peek(hotstuffProposal.Block.BlockID) == nil {
		log.Debug().Msg("block not found in cache, performing validation")
		// if last block is in cache it means that we can skip validation since it was already validated
		// otherwise we must validate it to proof validity of blocks range.
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
	}

	certifiedBatch, certifyingQC, err := c.pendingCache.AddBlocks(batch)
	if err != nil {
		return fmt.Errorf("could not add a range of pending blocks: %w", err)
	}

	log.Debug().Msgf("processing range resulted in %d certified blocks", len(certifiedBatch))

	if len(certifiedBatch) < 1 {
		return nil
	}

	// in-case we have already stopped our worker we use a select statement to avoid
	// blocking since there is no active consumer for this channel
	select {
	case c.certifiedBlocksChan <- rangeToCertifiedBlocks(certifiedBatch, certifyingQC):
	case <-c.ComponentManager.ShutdownSignal():
	}
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
			err := c.processFinalizedBlock(ctx, finalized) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		case blocks := <-c.certifiedBlocksChan:
			err := c.processCertifiedBlocks(ctx, blocks) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// OnFinalizedBlock updates local state of pendingCache tree using received finalized block.
// Is NOT concurrency safe, has to be used by the same goroutine as extendCertifiedBlocks.
// OnFinalizedBlock and extendCertifiedBlocks MUST be sequentially ordered.
func (c *Core) OnFinalizedBlock(final *flow.Header) {
	c.pendingCache.PruneUpToView(final.View)

	// in-case we have already stopped our worker we use a select statement to avoid
	// blocking since there is no active consumer for this channel
	select {
	case c.finalizedBlocksChan <- final:
	case <-c.ComponentManager.ShutdownSignal():
	}
}

// processCertifiedBlocks process a batch of certified blocks by adding them to the tree of pending blocks.
// As soon as tree returns a range of connected and certified blocks they will be added to the protocol state.
// Is NOT concurrency safe, has to be used by internal goroutine.
// No errors expected during normal operations.
func (c *Core) processCertifiedBlocks(ctx context.Context, blocks CertifiedBlocks) error {
	span, ctx := c.tracer.StartSpanFromContext(ctx, trace.FollowerProcessCertifiedBlocks)
	defer span.End()

	connectedBlocks, err := c.pendingTree.AddBlocks(blocks)
	if err != nil {
		return fmt.Errorf("could not process batch of certified blocks: %w", err)
	}
	err = c.extendCertifiedBlocks(ctx, connectedBlocks)
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}
	return nil
}

// extendCertifiedBlocks processes a connected range of certified blocks by applying them to protocol state.
// As result of this operation we might extend protocol state.
// Is NOT concurrency safe, has to be used by internal goroutine.
// No errors expected during normal operations.
func (c *Core) extendCertifiedBlocks(parentCtx context.Context, connectedBlocks CertifiedBlocks) error {
	span, parentCtx := c.tracer.StartSpanFromContext(parentCtx, trace.FollowerExtendCertifiedBlocks)
	defer span.End()

	for _, certifiedBlock := range connectedBlocks {
		span, ctx := c.tracer.StartBlockSpan(parentCtx, certifiedBlock.ID(), trace.FollowerExtendCertified)
		err := c.state.ExtendCertified(ctx, certifiedBlock.Block, certifiedBlock.QC)
		span.End()
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

// processFinalizedBlock processes new finalized block by applying to the PendingTree.
// Potentially PendingTree can resolve blocks that previously were not connected. Those blocks will be applied to the
// protocol state, resulting in extending length of chain.
// Is NOT concurrency safe, has to be used by internal goroutine.
// No errors expected during normal operations.
func (c *Core) processFinalizedBlock(ctx context.Context, finalized *flow.Header) error {
	span, ctx := c.tracer.StartSpanFromContext(ctx, trace.FollowerProcessFinalizedBlock)
	defer span.End()

	connectedBlocks, err := c.pendingTree.FinalizeFork(finalized)
	if err != nil {
		return fmt.Errorf("could not process finalized fork at view %d: %w", finalized.View, err)
	}
	err = c.extendCertifiedBlocks(ctx, connectedBlocks)
	if err != nil {
		return fmt.Errorf("could not extend protocol state during finalization: %w", err)
	}
	return nil
}

// rangeToCertifiedBlocks transform batch of connected blocks and a QC that certifies last block to a range of
// certified and connected blocks.
// Pure function.
func rangeToCertifiedBlocks(certifiedRange []*flow.Block, certifyingQC *flow.QuorumCertificate) CertifiedBlocks {
	certifiedBlocks := make(CertifiedBlocks, 0, len(certifiedRange))
	for i := 0; i < len(certifiedRange); i++ {
		block := certifiedRange[i]
		var qc *flow.QuorumCertificate
		if i < len(certifiedRange)-1 {
			qc = certifiedRange[i+1].Header.QuorumCertificate()
		} else {
			qc = certifyingQC
		}
		certifiedBlocks = append(certifiedBlocks, pending_tree.CertifiedBlock{
			Block: block,
			QC:    qc,
		})
	}
	return certifiedBlocks
}
