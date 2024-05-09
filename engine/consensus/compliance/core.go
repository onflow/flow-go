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
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
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
	finalizedView     counters.StrictMonotonousCounter
	finalizedHeight   counters.StrictMonotonousCounter
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
	c.ProcessFinalizedBlock(final)

	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

	return c, nil
}

// OnBlockProposal handles incoming block proposals.
// No errors are expected during normal operation. All returned exceptions
// are potential symptoms of internal state corruption and should be fatal.
func (c *Core) OnBlockProposal(proposal flow.Slashable[*messages.BlockProposal]) error {
	block := flow.Slashable[*flow.Block]{
		OriginID: proposal.OriginID,
		Message:  proposal.Message.Block.ToInternal(),
	}
	header := block.Message.Header
	blockID := header.ID()
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
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Str("traceID", traceID). // traceID is used to connect logs to traces
		Uint64("finalized_height", finalHeight).
		Uint64("finalized_view", finalView).
		Logger()
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
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization

	// ignore proposals that are already cached
	_, cached := c.pending.ByID(blockID)
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := c.headers.ByBlockID(blockID)
	if err == nil {
		log.Debug().Msg("skipping already processed proposal")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check proposal: %w", err)
	}

	// there are two possibilities if the proposal is neither already pending
	// processing in the cache, nor has already been processed:
	// 1) the proposal is unverifiable because the parent is unknown
	// => we cache the proposal
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if the parent is a pending block (disconnected from the incorporated state), we cache this block as well.
	// we don't have to request its parent block or its ancestor again, because as a
	// pending block, its parent block must have been requested.
	// if there was problem requesting its parent or ancestors, the sync engine's forward
	// syncing with range requests for finalized blocks will request for the blocks.
	_, found := c.pending.ByID(header.ParentID)
	if found {
		// add the block to the cache
		_ = c.pending.Add(block)
		c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	exists, err := c.headers.Exists(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not check parent exists: %w", err)
	}
	if !exists {
		_ = c.pending.Add(block)
		c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		c.sync.RequestBlock(header.ParentID, header.Height-1)
		log.Debug().Msg("requesting missing parent for proposal")
		return nil
	}

	// At this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not.
	// processBlockAndDescendants is a recursive function. Here we trace the
	// execution of the entire recursion, which might include processing the
	// proposal's pending children. There is another span within
	// processBlockProposal that measures the time spent for a single proposal.
	err = c.processBlockAndDescendants(block)
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
func (c *Core) processBlockAndDescendants(proposal flow.Slashable[*flow.Block]) error {
	header := proposal.Message.Header
	blockID := header.ID()

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
			err = c.voteAggregator.InvalidBlock(model.ProposalFromFlow(header))
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

	// drop all the children that should have been processed now
	c.pending.DropForParent(blockID)

	return nil
}

// processBlockProposal processes the given block proposal. The proposal must connect to
// the finalized state.
// Expected errors during normal operations:
//   - engine.OutdatedInputError if the block proposal is outdated (e.g. orphaned)
//   - model.InvalidProposalError if the block proposal is invalid
//   - engine.UnverifiableInputError if the block proposal cannot be verified
func (c *Core) processBlockProposal(proposal *flow.Block) error {
	startTime := time.Now()
	defer func() {
		c.hotstuffMetrics.BlockProcessingDuration(time.Since(startTime))
	}()

	header := proposal.Header
	blockID := header.ID()

	span, ctx := c.tracer.StartBlockSpan(context.Background(), blockID, trace.ConCompProcessBlockProposal)
	span.SetAttributes(
		attribute.String("proposer", header.ProposerID.String()),
	)
	defer span.End()

	hotstuffProposal := model.ProposalFromFlow(header)
	err := c.validator.ValidateProposal(hotstuffProposal)
	if err != nil {
		if model.IsInvalidProposalError(err) {
			return err
		}
		if errors.Is(err, model.ErrViewForUnknownEpoch) {
			// We have received a proposal, but we don't know the epoch its view is within.
			// We know:
			//  - the parent of this block is valid and was appended to the state (ie. we knew the epoch for it)
			//  - if we then see this for the child, one of two things must have happened:
			//    1. the proposer maliciously created the block for a view very far in the future (it's invalid)
			//      -> in this case we can disregard the block
			//    2. no blocks have been finalized within the epoch commitment deadline, and the epoch ended
			//       (breaking a critical assumption - see EpochCommitSafetyThreshold in protocol.Params for details)
			//      -> in this case, the network has encountered a critical failure
			//  - we assume in general that Case 2 will not happen, therefore this must be Case 1 - an invalid block
			return engine.NewUnverifiableInputError("unverifiable proposal with view from unknown epoch: %w", err)
		}
		return fmt.Errorf("unexpected error validating proposal: %w", err)
	}

	log := c.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Hex("parent_signer_indices", header.ParentVoterIndices).
		Logger()
	log.Info().Msg("processing block proposal")

	// see if the block is a valid extension of the protocol state
	block := &flow.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}
	err = c.state.Extend(ctx, block)
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
	log.Info().Msg("forwarding block proposal to hotstuff")
	c.hotstuff.SubmitProposal(hotstuffProposal)

	return nil
}

// ProcessFinalizedBlock performs pruning of stale data based on finalization event
// removes pending blocks below the finalized view
func (c *Core) ProcessFinalizedBlock(finalized *flow.Header) {
	// remove all pending blocks at or below the finalized view
	c.pending.PruneByView(finalized.View)
	c.finalizedHeight.Set(finalized.Height)
	c.finalizedView.Set(finalized.View)

	// always record the metric
	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
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
