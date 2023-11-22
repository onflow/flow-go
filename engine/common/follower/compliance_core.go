package follower

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine/common/follower/cache"
	"github.com/onflow/flow-go/engine/common/follower/pending_tree"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
)

// CertifiedBlocks is a connected list of certified blocks, in ascending height order.
type CertifiedBlocks []flow.CertifiedBlock

// defaultCertifiedRangeChannelCapacity maximum capacity of buffered channel that is used to transfer ranges of
// certified blocks to specific worker.
// Channel buffers ranges which consist of multiple blocks, so the real capacity of channel is larger
const defaultCertifiedRangeChannelCapacity = 20

// defaultFinalizedBlocksChannelCapacity maximum capacity of buffered channel that is used to transfer
// finalized blocks to specific worker.
const defaultFinalizedBlocksChannelCapacity = 10

// defaultPendingBlocksCacheCapacity maximum capacity of cache for pending blocks.
const defaultPendingBlocksCacheCapacity = 1000

// ComplianceCore implements main processing logic for follower engine.
// Generally is NOT concurrency safe but some functions can be used in concurrent setup.
type ComplianceCore struct {
	*component.ComponentManager
	log                       zerolog.Logger
	mempoolMetrics            module.MempoolMetrics
	tracer                    module.Tracer
	proposalViolationNotifier hotstuff.ProposalViolationConsumer
	pendingCache              *cache.Cache
	pendingTree               *pending_tree.PendingTree
	state                     protocol.FollowerState
	follower                  module.HotStuffFollower
	validator                 hotstuff.Validator
	sync                      module.BlockRequester
	certifiedRangesChan       chan CertifiedBlocks // delivers ranges of certified blocks to main core worker
	finalizedBlocksChan       chan *flow.Header    // delivers finalized blocks to main core worker.
}

var _ complianceCore = (*ComplianceCore)(nil)

// NewComplianceCore creates new instance of ComplianceCore.
// No errors expected during normal operations.
func NewComplianceCore(log zerolog.Logger,
	mempoolMetrics module.MempoolMetrics,
	heroCacheCollector module.HeroCacheMetrics,
	followerConsumer hotstuff.FollowerConsumer,
	state protocol.FollowerState,
	follower module.HotStuffFollower,
	validator hotstuff.Validator,
	sync module.BlockRequester,
	tracer module.Tracer,
) (*ComplianceCore, error) {
	finalizedBlock, err := state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not query finalized block: %w", err)
	}

	c := &ComplianceCore{
		log:                       log.With().Str("engine", "follower_core").Logger(),
		mempoolMetrics:            mempoolMetrics,
		state:                     state,
		proposalViolationNotifier: followerConsumer,
		pendingCache:              cache.NewCache(log, defaultPendingBlocksCacheCapacity, heroCacheCollector, followerConsumer),
		pendingTree:               pending_tree.NewPendingTree(finalizedBlock),
		follower:                  follower,
		validator:                 validator,
		sync:                      sync,
		tracer:                    tracer,
		certifiedRangesChan:       make(chan CertifiedBlocks, defaultCertifiedRangeChannelCapacity),
		finalizedBlocksChan:       make(chan *flow.Header, defaultFinalizedBlocksChannelCapacity),
	}

	// prune cache to latest finalized view
	c.pendingCache.PruneUpToView(finalizedBlock.View)

	c.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(c.processCoreSeqEvents).
		Build()

	return c, nil
}

// OnBlockRange processes a range of connected blocks. It validates the incoming batch, adds it to cache of pending
// blocks and schedules certified blocks for further processing. The input list must be sequentially ordered forming
// a chain, i.e. connectedRange[i] is the parent of connectedRange[i+1]. Submitting a disconnected batch results in
// an `ErrDisconnectedBatch` error and the batch is dropped (no-op).
// This method is safe to use in concurrent environment.
// Caution: method might block if internally too many certified blocks are queued in the channel `certifiedRangesChan`.
// Expected errors during normal operations:
//   - cache.ErrDisconnectedBatch
func (c *ComplianceCore) OnBlockRange(originID flow.Identifier, batch []*flow.Block) error {
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
		// Caution: we are _not_ checking the proposal's full validity here. Instead, we need to check
		// the following two critical properties:
		// 1. The block has been signed by the legitimate primary for the view. This is important in case
		//    there are multiple blocks for the view. We need to differentiate the following byzantine cases:
		//     (i) Some other consensus node that is _not_ primary is trying to publish a block.
		//         This would result in the validation below failing with an `InvalidProposalError`.
		//    (ii) The legitimate primary for the view is equivocating. In this case, the validity check
		//         below would pass. Though, the `PendingTree` would eventually notice this, when we connect
		//         the equivocating blocks to the latest finalized block.
		// 2. The QC within the block is valid. A valid QC proves validity of all ancestors.
		err := c.validator.ValidateProposal(hotstuffProposal)
		if err != nil {
			if invalidBlockError, ok := model.AsInvalidProposalError(err); ok {
				c.proposalViolationNotifier.OnInvalidBlockDetected(flow.Slashable[model.InvalidProposalError]{
					OriginID: originID,
					Message:  *invalidBlockError,
				})
				return nil
			}
			if errors.Is(err, model.ErrViewForUnknownEpoch) {
				// We have received a proposal, but we don't know the epoch its view is within.
				// Conceptually, there are three scenarios that could lead to this edge-case:
				//  1. the proposer maliciously created the block for a view very far in the future (it's invalid)
				//     -> in this case we can disregard the block
				//  2. This node is very far behind and hasn't processed enough blocks to observe the EpochCommit
				//     service event.
				//     -> in this case we can disregard the block
				//     Note: we could eliminate this edge case by dropping future blocks, iff their _view_
				//           is strictly larger than `V + EpochCommitSafetyThreshold`, where `V` denotes
				//           the latest finalized block known to this node.
				//  3. No blocks have been finalized for the last `EpochCommitSafetyThreshold` views. This breaks
				//     a critical liveness assumption - see EpochCommitSafetyThreshold in protocol.Params for details.
				//     -> In this case, it is ok for the protocol to halt. Consequently, we can just disregard
				//        the block, which will probably lead to this node eventually halting.
				log.Err(err).Msg("unable to validate proposal with view from unknown epoch")
				return nil
			}
			return fmt.Errorf("unexpected error validating proposal: %w", err)
		}
	}

	certifiedBatch, certifyingQC, err := c.pendingCache.AddBlocks(batch)
	if err != nil {
		return fmt.Errorf("could not add a range of pending blocks: %w", err) // ErrDisconnectedBatch or exception
	}
	log.Debug().Msgf("caching block range resulted in %d certified blocks (possibly including additional cached blocks)", len(certifiedBatch))

	if len(certifiedBatch) < 1 {
		return nil
	}
	certifiedRange, err := rangeToCertifiedBlocks(certifiedBatch, certifyingQC)
	if err != nil {
		return fmt.Errorf("converting the certified batch to list of certified blocks failed: %w", err)
	}

	// in case we have already stopped our worker, we use a select statement to avoid
	// blocking since there is no active consumer for this channel
	select {
	case c.certifiedRangesChan <- certifiedRange:
	case <-c.ComponentManager.ShutdownSignal():
	}
	return nil
}

// processCoreSeqEvents processes events that need to be dispatched on dedicated core's goroutine.
// Here we process events that need to be sequentially ordered(processing certified blocks and new finalized blocks).
// Implements `component.ComponentWorker` signature.
// Is NOT concurrency safe: should be executed by _single dedicated_ goroutine.
func (c *ComplianceCore) processCoreSeqEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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
		case blocks := <-c.certifiedRangesChan:
			err := c.processCertifiedBlocks(ctx, blocks) // no errors expected during normal operations
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// OnFinalizedBlock updates local state of pendingCache tree using received finalized block and queues finalized block
// to be processed by internal goroutine.
// This function is safe to use in concurrent environment.
// CAUTION: this function blocks and hence is not compliant with the `FinalizationConsumer.OnFinalizedBlock` interface.
func (c *ComplianceCore) OnFinalizedBlock(final *flow.Header) {
	c.pendingCache.PruneUpToView(final.View)

	// in-case we have already stopped our worker we use a select statement to avoid
	// blocking since there is no active consumer for this channel
	select {
	case c.finalizedBlocksChan <- final:
	case <-c.ComponentManager.ShutdownSignal():
	}
}

// processCertifiedBlocks processes the batch of certified blocks:
//  1. We add the certified blocks to the PendingTree. This might causes the pending PendingTree to detect
//     additional blocks as now being connected to the latest finalized block. Specifically, the PendingTree
//     returns the list `connectedBlocks`, which contains the subset of `blocks` that are connect to the
//     finalized block plus all of their connected descendants. The list `connectedBlocks` is in 'parent first'
//     order, i.e. a block is listed before any of its descendants. The PendingTree guarantees that all
//     ancestors are listed, _unless_ the ancestor is the finalized block or the ancestor has been returned
//     by a previous call to `PendingTree.AddBlocks`.
//  2. We extend the protocol state with the connected certified blocks from step 1.
//  3. We submit the connected certified blocks from step 1 to the consensus follower.
//
// Is NOT concurrency safe: should be executed by _single dedicated_ goroutine.
// No errors expected during normal operations.
func (c *ComplianceCore) processCertifiedBlocks(ctx context.Context, blocks CertifiedBlocks) error {
	span, ctx := c.tracer.StartSpanFromContext(ctx, trace.FollowerProcessCertifiedBlocks)
	defer span.End()

	// Step 1: add blocks to our PendingTree of certified blocks
	pendingTreeSpan, _ := c.tracer.StartSpanFromContext(ctx, trace.FollowerExtendPendingTree)
	connectedBlocks, err := c.pendingTree.AddBlocks(blocks)
	pendingTreeSpan.End()
	if err != nil {
		return fmt.Errorf("could not process batch of certified blocks: %w", err)
	}

	// Step 2 & 3: extend protocol state with connected certified blocks and forward them to consensus follower
	for _, certifiedBlock := range connectedBlocks {
		s, _ := c.tracer.StartBlockSpan(ctx, certifiedBlock.ID(), trace.FollowerExtendProtocolState)
		err = c.state.ExtendCertified(ctx, certifiedBlock.Block, certifiedBlock.CertifyingQC)
		s.End()
		if err != nil {
			return fmt.Errorf("could not extend protocol state with certified block: %w", err)
		}

		b, err := model.NewCertifiedBlock(model.BlockFromFlow(certifiedBlock.Block.Header), certifiedBlock.CertifyingQC)
		if err != nil {
			return fmt.Errorf("failed to convert certified block %v to HotStuff type: %w", certifiedBlock.Block.ID(), err)
		}
		c.follower.AddCertifiedBlock(&b) // submit the model to follower for async processing
	}
	return nil
}

// processFinalizedBlock informs the PendingTree about finalization of the given block.
// Is NOT concurrency safe: should be executed by _single dedicated_ goroutine.
// No errors expected during normal operations.
func (c *ComplianceCore) processFinalizedBlock(ctx context.Context, finalized *flow.Header) error {
	span, _ := c.tracer.StartSpanFromContext(ctx, trace.FollowerProcessFinalizedBlock)
	defer span.End()

	connectedBlocks, err := c.pendingTree.FinalizeFork(finalized)
	if err != nil {
		return fmt.Errorf("could not process finalized fork at view %d: %w", finalized.View, err)
	}
	// The pending tree allows to skip ahead, which makes the algorithm more general and simplifies its implementation.
	// However, here we are implementing the consensus follower, which cannot skip ahead. This is because the consensus
	// follower locally determines finality and therefore must ingest every block. In other words: ever block that is
	// later finalized must have been connected before. Otherwise, the block would never have been forwarded to the
	// HotStuff follower and no finalization notification would have been triggered.
	// Therefore, from the perspective of the consensus follower, receiving a _non-empty_ `connectedBlocks` is a
	// symptom of internal state corruption or a bug.
	if len(connectedBlocks) > 0 {
		return fmt.Errorf("finalizing block %v caused the PendingTree to connect additional blocks, which is a symptom of internal state corruption or a bug", finalized.ID())
	}
	return nil
}

// rangeToCertifiedBlocks transform batch of connected blocks and a QC that certifies last block to a range of
// certified and connected blocks.
// Pure function (side-effect free). No errors expected during normal operations.
func rangeToCertifiedBlocks(certifiedRange []*flow.Block, certifyingQC *flow.QuorumCertificate) (CertifiedBlocks, error) {
	certifiedBlocks := make(CertifiedBlocks, 0, len(certifiedRange))
	lastIndex := len(certifiedRange) - 1
	for i, block := range certifiedRange {
		var qc *flow.QuorumCertificate
		if i < lastIndex {
			qc = certifiedRange[i+1].Header.QuorumCertificate()
		} else {
			qc = certifyingQC
		}

		// bundle block and its certifying QC to `CertifiedBlock`:
		certBlock, err := flow.NewCertifiedBlock(block, qc)
		if err != nil {
			return nil, fmt.Errorf("constructing certified root block failed: %w", err)
		}
		certifiedBlocks = append(certifiedBlocks, certBlock)
	}
	return certifiedBlocks, nil
}
