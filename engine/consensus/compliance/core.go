package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Core contains the central business logic for the main consensus' compliance engine.
// It is responsible for handling communication for the embedded consensus algorithm.
// CAUTION with CONCURRENCY:
//   - At the moment, compliance.Core _can not_ process blocks concurrently. Callers of `OnBlockProposal`
//     need to ensure single-threaded access.
//   - The only exception is calls to `ProcessFinalizedView`, which is the only concurrency-safe
//     method of compliance.Core
type Core struct {
	log                       zerolog.Logger // used to log relevant actions with context
	config                    compliance.Config
	engineMetrics             module.EngineMetrics
	mempoolMetrics            module.MempoolMetrics
	hotstuffMetrics           module.HotstuffMetrics
	complianceMetrics         module.ComplianceMetrics
	proposalViolationNotifier hotstuff.ProposalViolationConsumer
	tracer                    module.Tracer
	headers                   storage.Headers
	payloads                  storage.Payloads
	state                     protocol.ParticipantState
	// track latest finalized view/height - used to efficiently drop outdated or too-far-ahead blocks
	finalizedView     counters.StrictMonotonicCounter
	finalizedHeight   counters.StrictMonotonicCounter
	pending           module.PendingBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
	validator         hotstuff.Validator
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
}

// NewCore instantiates the business logic for the main consensus' compliance engine.
func NewCore(
	log zerolog.Logger,
	collector module.EngineMetrics,
	mempool module.MempoolMetrics,
	hotstuffMetrics module.HotstuffMetrics,
	complianceMetrics module.ComplianceMetrics,
	proposalViolationNotifier hotstuff.ProposalViolationConsumer,
	tracer module.Tracer,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.ParticipantState,
	pending module.PendingBlockBuffer,
	sync module.BlockRequester,
	validator hotstuff.Validator,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	config compliance.Config,
) (*Core, error) {

	c := &Core{
		log:                       log.With().Str("compliance", "core").Logger(),
		config:                    config,
		engineMetrics:             collector,
		tracer:                    tracer,
		mempoolMetrics:            mempool,
		hotstuffMetrics:           hotstuffMetrics,
		complianceMetrics:         complianceMetrics,
		proposalViolationNotifier: proposalViolationNotifier,
		headers:                   headers,
		payloads:                  payloads,
		state:                     state,
		pending:                   pending,
		sync:                      sync,
		hotstuff:                  hotstuff,
		validator:                 validator,
		voteAggregator:            voteAggregator,
		timeoutAggregator:         timeoutAggregator,
	}

	// initialize finalized boundary cache
	final, err := c.state.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("could not initialized finalized boundary cache: %w", err)
	}
	err = c.ProcessFinalizedBlock(final)
	if err != nil {
		return nil, fmt.Errorf("could not process finalized block: %w", err)
	}

	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

	return c, nil
}

// OnBlockProposal handles incoming basic structural validated block proposals.
// No errors are expected during normal operation. All returned exceptions
// are potential symptoms of internal state corruption and should be fatal.
func (c *Core) OnBlockProposal(proposal flow.Slashable[*flow.Proposal]) error {
	// In general, there are a variety of attacks that a byzantine proposer might attempt. Conceptually,
	// there are two classes:
	//	 (I) Protocol-level attacks by either sending individually invalid blocks, or by equivocating with
	//       by sending a pair of conflicting blocks (individually valid and/or invalid) for the same view.
	//	(II) Resource exhaustion attacks by sending a large number of individually invalid or valid blocks.
	// Category (I) will be eventually detected. Attacks of category (II), typically contain elements of (I).
	// This is because the protocol is purposefully designed such that there is few degrees of freedom available
	// to a byzantine proposer attempting to mount a resource exhaustion attack (type II), unless the proposer
	// violates protocol rules i.e. provides evidence of their wrongdoing (type I).
	// However, we have to make sure that the nodes survive an attack of category (II), and stays responsive.
	// If the node crashes, the node will lose the evidence of the attack, and the byzantine proposer
	// will have succeeded in their goal of mounting a denial-of-service attack without being held accountable.
	//
	// The general requirements for a BFT system are:
	// 1. withstand attacks (don't crash and remain responsive)
	// 2. detect attacks and collect evidence for slashing challenges
	// 3. suppress the attack by slashing and ejecting the offending node(s)
	//
	// The primary responsibility of compliance engine is to protect the business logic from attacks of
	// category (II) and to collect evidence for attacks of category (I) for blocks that are _individually_
	// invalid. The compliance engine may detect some portion of equivocation attacks (type I), in order
	// to better protect itself from resource exhaustion attacks (type II). Though, the primary responsibility
	// for detecting equivocation is with the hotstuff layer. The reason is that, in case of equivocation with
	// multiple valid blocks, the compliance engine can't know which block might get certified and potentially
	// finalized. So it can't reject _valid_ equivocating blocks outright, as that might lead to liveness issues.
	//
	// The compliance engine must be resilient to the following classes of resource exhaustion attacks:
	//  1. A byzantine proposers might attempt to create blocks at many different future views. Mitigations:
	//     • Only proposals whose proposer is the valid leader for the respective view should pass the compliance
	//     engine. Block that are not proposed by a valid leader are outright reject, and we create a slashing
	//     challenge against the proposer. This filtering should be done by the compliance engine. Such blocks
	//     should never reach the higher-level business logic.
	//     • A byzantine proposer might attempt to create blocks for a large number of different future views,
	//     for which it is valid leader. This is mitigated by dropping blocks that are too far ahead of the
	//     locally finalized view. The threshold is configured via `SkipNewProposalsThreshold` parameter.
	//     This does not lead to a slashing challenge, as we can't reliably detect without investing significant
	//     CPU resources validating the QC, whether the proposer is violating protocol rules by making up an
	//     invalid QC / TC. Valid blocks will eventually be retrieved via sync again, once the local finalized
	//     view catches up, even if they were dropped at first.
	//  2. A byzantine proposers might spam us with many different _valid_ blocks for the same view, for which
	//     it is the leader. This is particularly dangerous since each block is valid, and we don't know which block
	//     will get certified so for protocol liveness we need to store them all. This leads to a potentially
	//     unlimited number of valid blocks that are ingested in the cache of pending blocks which will lead to
	//     a memory overflow and panic. To prevent this our data structure([PendingBlockBuffer]) accepts proposals
	//     only in a limited view window, this prevents growing the tree of pending blocks in depth, the depth is limited
	//     by the window size. The width of the tree is limited by storing only a single proposal per view.
	//     Strictly speaking this is not enough since if other proposal got certified we can't make progress. To deal
	//     with this situation we rely on syncing of certified blocks. Eventually, a certified block will be synced
	//     and even though we have stored other proposal for that view, we will still be able to make progress since
	//     we have obtained the certified block(at most one block per view can get certified).
	//

	block := proposal.Message.Block
	header := block.ToHeader()
	blockID := block.ID()
	finalHeight := c.finalizedHeight.Value()
	finalView := c.finalizedView.Value()

	span, _ := c.tracer.StartBlockSpan(context.Background(), header.ID(), trace.CONCompOnBlockProposal)
	span.SetAttributes(
		attribute.Int64("view", int64(header.View)),
		attribute.String("origin_id", proposal.OriginID.String()),
		attribute.String("proposer", header.ProposerID.String()),
	)
	traceID := span.SpanContext().TraceID().String()
	defer span.End()

	log := c.log.With().
		Hex("origin_id", proposal.OriginID[:]).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Hex("proposer", header.ProposerID[:]).
		Time("timestamp", time.UnixMilli(int64(header.Timestamp)).UTC()).
		Logger()
	if log.Debug().Enabled() {
		log = log.With().
			Uint64("finalized_height", finalHeight).
			Uint64("finalized_view", finalView).
			Str("chain_id", header.ChainID.String()).
			Hex("payload_hash", header.PayloadHash[:]).
			Hex("parent_signer_indices", header.ParentVoterIndices).
			Str("traceID", traceID). // traceID is used to connect logs to traces
			Logger()
	}
	log.Info().Msg("block proposal received")

	// drop proposals below the finalized threshold
	if header.Height <= finalHeight || header.View <= finalView {
		log.Debug().Msg("dropping block below finalized boundary")
		return nil
	}

	skipNewProposalsThreshold := c.config.GetSkipNewProposalsThreshold()
	// ignore proposals which are too far ahead of our local finalized state
	// instead, rely on sync engine to catch up finalization more effectively, and avoid
	// large subtree of blocks to be cached.
	if header.View > finalView+skipNewProposalsThreshold {
		log.Debug().
			Uint64("skip_new_proposals_threshold", skipNewProposalsThreshold).
			Msg("dropping block too far ahead of locally finalized view")
		return nil
	}

	// first, we reject all blocks that we don't need to process:
	// 1. blocks already in the cache, that are disconnected: they will be processed later.
	// 2. blocks already in the cache, that were already processed: they will be eventually pruned by view.
	// 3. blocks already on disk: they were processed and await finalization

	// 1,2. To prevent memory exhaustion attacks we store single proposal per view, so we can ignore
	// all other proposals if we have already cached something.
	blocksByView := c.pending.ByView(block.View)
	if len(blocksByView) > 0 {
		log.Debug().Msg("skipping proposal since we have already processed one for given view")
		return nil
	}

	// 3. Ignore proposals that were already processed
	_, err := c.headers.ByBlockID(blockID)
	if err == nil {
		log.Debug().Msg("skipping already processed proposal")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check proposal: %w", err)
	}

	// Each proposal is validated before being cached or processed, this results in a statement:
	// - cache stores only valid proposals(from hotstuff's perspective).
	// A malicious leader might try to send many proposals both valid and invalid to drain CPU resources
	// by validating proposals. Consider two cases:
	// 1. Leader sends multiple invalid blocks: this is a one time attack since it's even one block provides a slashing evidence
	// 	  which results in immediate slashing. This attack is very short living and expensive.
	// 2. Leader sends multiple valid blocks: this is prevented by storing single block per view and accepting proposals in
	//    specific view range. This attack has a very limited surface and power.
	hotstuffProposal := model.SignedProposalFromBlock(proposal.Message)
	err = c.validator.ValidateProposal(hotstuffProposal)
	if err != nil {
		if invalidBlockErr, ok := model.AsInvalidProposalError(err); ok {
			log.Err(err).Msg("received invalid block from other node (potential slashing evidence?)")

			// notify consumers about invalid block
			c.proposalViolationNotifier.OnInvalidBlockDetected(flow.Slashable[model.InvalidProposalError]{
				OriginID: proposal.OriginID,
				Message:  *invalidBlockErr,
			})

			// notify VoteAggregator about the invalid block
			err = c.voteAggregator.InvalidBlock(model.SignedProposalFromBlock(proposal.Message))
			if err != nil {
				if mempool.IsBelowPrunedThresholdError(err) {
					log.Warn().Msg("received invalid block, but is below pruned threshold")
					return nil
				}
				return fmt.Errorf("unexpected error notifying vote aggregator about invalid block: %w", err)
			}
			return nil
		}
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			// generally speaking we shouldn't get to this point as SkipNewProposalsThreshold controls
			// how many views in advance we accept a block, but in case it's very large we might get into a situation
			// where we don't anything about the epoch.
			return nil
		}
		return fmt.Errorf("unexpected error validating proposal: %w", err)
	}

	// At this point we are dealing with a block proposal that is neither present in the cache nor on disk.
	// There are three possibilities if the proposal is stored neither in cache nor on disk:
	// 1. The proposal is connected to the finalized state => we perform the further processing and pass it to the hotstuff layer.
	// 2. The proposal is not connected to the finalized state:
	// 2.1 Parent has been already cached, meaning we have a partial chain => cache the proposal and wait for eventual resolution of missing the piece.
	// 2.2 Parent has not been cached yet => cache the proposal, additionally request the missing parent from the committee.

	// 1. Check if we parent is connected to the finalized state
	exists, err := c.headers.Exists(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not check parent exists: %w", err)
	}
	if !exists {
		// 2. Cache the proposal either way for 2.1 or 2.2
		if err := c.pending.Add(proposal); err != nil {
			if mempool.IsBeyondActiveRangeError(err) {
				// In general, we expect the block buffer to use SkipNewProposalsThreshold,
				// however since it is instantiated outside this component, we allow the thresholds to differ
				log.Debug().Err(err).Msg("dropping block beyond block buffer active range")
				return nil
			}
			return fmt.Errorf("could not add proposal to pending buffer: %w", err)
		}
		c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		// 2.2 Parent has not been cached yet, request it from the committee
		if _, found := c.pending.ByID(header.ParentID); !found {
			c.sync.RequestBlock(header.ParentID, header.Height-1)
			log.Debug().Msg("requesting missing parent for proposal")
		}
		return nil
	}

	// 1. At this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not.
	// processBlockAndDescendants is a recursive function. Here we trace the
	// execution of the entire recursion, which might include processing the
	// proposal's pending children. There is another span within
	// processBlockProposal that measures the time spent for a single proposal.
	err = c.processBlockAndDescendants(proposal)
	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	return nil
}

// processBlockAndDescendants is a recursive function that processes a block and
// its pending proposals for its children. By induction, any children connected
// to a valid proposal are validly connected to the finalized state and can be
// processed as well.
// No errors are expected during normal operation. All returned exceptions
// are potential symptoms of internal state corruption and should be fatal.
func (c *Core) processBlockAndDescendants(proposal flow.Slashable[*flow.Proposal]) error {
	block := proposal.Message.Block
	header := block.ToHeader()
	blockID := block.ID()

	log := c.log.With().
		Str("block_id", blockID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Uint64("parent_view", header.ParentView).
		Logger()

	// process block itself
	err := c.processBlockProposal(proposal.Message)
	if err != nil {
		if checkForAndLogOutdatedInputError(err, log) || checkForAndLogUnverifiableInputError(err, log) {
			return nil
		}
		if invalidBlockErr, ok := model.AsInvalidProposalError(err); ok {
			log.Err(err).Msg("received invalid block from other node (potential slashing evidence?)")

			// notify consumers about invalid block
			c.proposalViolationNotifier.OnInvalidBlockDetected(flow.Slashable[model.InvalidProposalError]{
				OriginID: proposal.OriginID,
				Message:  *invalidBlockErr,
			})

			// notify VoteAggregator about the invalid block
			err = c.voteAggregator.InvalidBlock(model.SignedProposalFromBlock(proposal.Message))
			if err != nil {
				if mempool.IsBelowPrunedThresholdError(err) {
					log.Warn().Msg("received invalid block, but is below pruned threshold")
					return nil
				}
				return fmt.Errorf("unexpected error notifying vote aggregator about invalid block: %w", err)
			}
			return nil
		}
		// unexpected error: potentially corrupted internal state => abort processing and escalate error
		return fmt.Errorf("failed to process block %x: %w", blockID, err)
	}

	// process all children
	// do not break on invalid or outdated blocks as they should not prevent us
	// from processing other valid children
	children, has := c.pending.ByParentID(blockID)
	if !has {
		return nil
	}
	for _, child := range children {
		cpr := c.processBlockAndDescendants(child)
		if cpr != nil {
			// unexpected error: potentially corrupted internal state => abort processing and escalate error
			return cpr
		}
	}

	return nil
}

// processBlockProposal processes the given block proposal. The proposal must connect to
// the finalized state.
// Expected errors during normal operations:
//   - engine.OutdatedInputError if the block proposal is outdated (e.g. orphaned)
//   - model.InvalidProposalError if the block proposal is invalid
//   - engine.UnverifiableInputError if the block proposal cannot be verified
func (c *Core) processBlockProposal(proposal *flow.Proposal) error {
	startTime := time.Now()
	defer func() {
		c.hotstuffMetrics.BlockProcessingDuration(time.Since(startTime))
	}()

	block := proposal.Block
	header := block.ToHeader()
	blockID := block.ID()

	span, ctx := c.tracer.StartBlockSpan(context.Background(), blockID, trace.ConCompProcessBlockProposal)
	span.SetAttributes(
		attribute.String("proposer", header.ProposerID.String()),
	)
	defer span.End()

	log := c.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", time.UnixMilli(int64(header.Timestamp)).UTC()).
		Hex("proposer", header.ProposerID[:]).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Logger()
	log.Debug().Msg("processing block proposal")

	hotstuffProposal := model.SignedProposalFromBlock(proposal)
	// see if the block is a valid extension of the protocol state
	err := c.state.Extend(ctx, proposal)
	if err != nil {
		if state.IsInvalidExtensionError(err) {
			// if the block proposes an invalid extension of the protocol state, then the block is invalid
			return model.NewInvalidProposalErrorf(hotstuffProposal, "invalid extension of protocol state (block: %x, height: %d): %w", blockID, header.Height, err)
		}
		if state.IsOutdatedExtensionError(err) {
			// protocol state aborted processing of block as it is on an abandoned fork: block is outdated
			return engine.NewOutdatedInputErrorf("outdated extension of protocol state: %w", err)
		}
		// unexpected error: potentially corrupted internal state => abort processing and escalate error
		return fmt.Errorf("unexpected exception while extending protocol state with block %x at height %d: %w", blockID, header.Height, err)
	}

	// notify vote aggregator about a new block, so that it can start verifying
	// votes for it.
	c.voteAggregator.AddBlock(hotstuffProposal)

	// submit the model to hotstuff for processing
	// TODO replace with pubsub https://github.com/dapperlabs/flow-go/issues/6395
	log.Debug().Msg("forwarding block proposal to hotstuff")
	c.hotstuff.SubmitProposal(hotstuffProposal)

	return nil
}

// ProcessFinalizedBlock performs pruning of stale data based on finalization event
// removes pending blocks below the finalized view
// No errors are expected during normal operation.
func (c *Core) ProcessFinalizedBlock(finalized *flow.Header) error {
	// remove all pending blocks at or below the finalized view
	err := c.pending.PruneByView(finalized.View)
	if err != nil {
		return err
	}
	c.finalizedHeight.Set(finalized.Height)
	c.finalizedView.Set(finalized.View)

	// always record the metric
	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
	return nil
}

// checkForAndLogOutdatedInputError checks whether error is an `engine.OutdatedInputError`.
// If this is the case, we emit a log message and return true.
// For any error other than `engine.OutdatedInputError`, this function is a no-op
// and returns false.
func checkForAndLogOutdatedInputError(err error, log zerolog.Logger) bool {
	if engine.IsOutdatedInputError(err) {
		// child is outdated by the time we started processing it
		// => node was probably behind and is catching up. Log as warning
		log.Info().Msg("dropped processing of abandoned fork; this might be an indicator that the node is slightly behind")
		return true
	}
	return false
}

// checkForAndLogUnverifiableInputError checks whether error is an `engine.UnverifiableInputError`.
// If this is the case, we emit a log message and return true.
// For any error other than `engine.UnverifiableInputError`, this function is a no-op
// and returns false.
func checkForAndLogUnverifiableInputError(err error, log zerolog.Logger) bool {
	if engine.IsUnverifiableInputError(err) {
		// the block cannot be validated
		log.Warn().Err(err).Msg("received unverifiable block proposal; " +
			"this might be an indicator that a malicious proposer is generating detached blocks very far ahead")
		return true
	}
	return false
}
