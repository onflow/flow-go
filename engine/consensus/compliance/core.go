// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

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
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core contains the central business logic for the main consensus' compliance engine.
// It is responsible for handling communication for the embedded consensus algorithm.
// NOTE: Core is designed to be non-thread safe and cannot be used in concurrent environment
// user of this object needs to ensure single thread access.
type Core struct {
	log               zerolog.Logger // used to log relevant actions with context
	config            compliance.Config
	engineMetrics     module.EngineMetrics
	mempoolMetrics    module.MempoolMetrics
	complianceMetrics module.ComplianceMetrics
	tracer            module.Tracer
	cleaner           storage.Cleaner
	headers           storage.Headers
	payloads          storage.Payloads
	state             protocol.MutableState
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
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	complianceMetrics module.ComplianceMetrics,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.MutableState,
	pending module.PendingBlockBuffer,
	sync module.BlockRequester,
	validator hotstuff.Validator,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	opts ...compliance.Opt,
) (*Core, error) {

	config := compliance.DefaultConfig()
	for _, apply := range opts {
		apply(&config)
	}

	e := &Core{
		log:               log.With().Str("compliance", "core").Logger(),
		config:            config,
		engineMetrics:     collector,
		tracer:            tracer,
		mempoolMetrics:    mempool,
		complianceMetrics: complianceMetrics,
		cleaner:           cleaner,
		headers:           headers,
		payloads:          payloads,
		state:             state,
		pending:           pending,
		sync:              sync,
		hotstuff:          nil, // use `WithConsensus`
		validator:         validator,
		voteAggregator:    voteAggregator,
		timeoutAggregator: timeoutAggregator,
	}
	e.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	return e, nil
}

// OnBlockProposal handles incoming block proposals.
// No errors are expected during normal operation. All returned exceptions
// are potential symptoms of internal state corruption and should be fatal.
func (c *Core) OnBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	var traceID string

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), proposal.Header.ID(), trace.CONCompOnBlockProposal)
	if isSampled {
		span.SetAttributes(
			attribute.Int64("view", int64(proposal.Header.View)),
			attribute.String("origin_id", originID.String()),
			attribute.String("proposer", proposal.Header.ProposerID.String()),
		)
		traceID = span.SpanContext().TraceID().String()
	}
	defer span.End()

	header := proposal.Header
	blockID := header.ID()
	log := c.log.With().
		Hex("origin_id", originID[:]).
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
		Logger()
	log.Info().Msg("block proposal received")

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

	// ignore proposals which are too far ahead of our local finalized state
	// instead, rely on sync engine to catch up finalization more effectively, and avoid
	// large subtree of blocks to be cached.
	final, err := c.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get latest finalized header: %w", err)
	}
	if header.Height > final.Height && header.Height-final.Height > c.config.SkipNewProposalsThreshold {
		log.Debug().
			Uint64("final_height", final.Height).
			Msg("dropping block too far ahead of locally finalized height")
		return nil
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
		_ = c.pending.Add(originID, proposal)
		c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = c.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {
		_ = c.pending.Add(originID, proposal)
		c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		c.sync.RequestBlock(header.ParentID, header.Height-1)
		log.Debug().Msg("requesting missing parent for proposal")

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	// At this point, we should be able to connect the proposal to the finalized
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

	// most of the heavy database checks are done at this point, so this is a
	// good moment to potentially kick-off a garbage collection of the DB
	// NOTE: this is only effectively run every 1000th calls, which corresponds
	// to every 1000th successfully processed block
	c.cleaner.RunGC()

	return nil
}

// processBlockAndDescendants is a recursive function that processes a block and
// its pending proposals for its children. By induction, any children connected
// to a valid proposal are validly connected to the finalized state and can be
// processed as well.
// No errors are expected during normal operation. All returned exceptions
// are potential symptoms of internal state corruption and should be fatal.
func (c *Core) processBlockAndDescendants(proposal *messages.BlockProposal) error {
	blockID := proposal.Header.ID()

	log := c.log.With().
		Str("block_id", blockID.String()).
		Uint64("block_height", proposal.Header.Height).
		Uint64("block_view", proposal.Header.View).
		Logger()

	// process block itself
	err := c.processBlockProposal(proposal)
	if err != nil {
		if engine.IsOutdatedInputError(err) {
			// child is outdated by the time we started processing it
			// => node was probably behind and is catching up. Log as warning
			log.Info().Msg("dropped processing of abandoned fork; this might be an indicator that the node is slightly behind")
			return nil
		}
		if engine.IsInvalidInputError(err) {
			// the block is invalid; log as error as we desire honest participation
			// ToDo: potential slashing
			log.Warn().Err(err).Msg("received invalid block from other node (potential slashing evidence?)")
			return nil
		}
		if engine.IsUnverifiableInputError(err) {
			// the block cannot be validated
			// TODO: potential slashing
			log.Err(err).Msg("received unverifiable block proposal; this is an indicator of an invalid (slashable) proposal or an epoch failure")
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
		childProposal := &messages.BlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		cpr := c.processBlockAndDescendants(childProposal)
		if cpr != nil {
			// unexpected error: potentially corrupted internal state => abort processing and escalate error
			return cpr
		}
	}

	// drop all of the children that should have been processed now
	c.pending.DropForParent(blockID)

	return nil
}

// processBlockProposal processes the given block proposal. The proposal must connect to
// the finalized state.
// Expected errors during normal operations:
//   - engine.OutdatedInputError if the block proposal is outdated (e.g. orphaned)
//   - engine.InvalidInputError if the block proposal is invalid
//   - engine.UnverifiableInputError if the proposal cannot be validated
func (c *Core) processBlockProposal(proposal *messages.BlockProposal) error {
	startTime := time.Now()
	defer c.complianceMetrics.BlockProposalDuration(time.Since(startTime))

	header := proposal.Header
	blockID := header.ID()

	span, ctx, isSampled := c.tracer.StartBlockSpan(context.Background(), blockID, trace.ConCompProcessBlockProposal)
	if isSampled {
		span.SetAttributes(
			attribute.String("proposer", header.ProposerID.String()),
		)
	}
	defer span.End()

	// retrieve the parent block
	//  - once we reach this point, the parent block must have been validated
	//    and inserted to the protocol state
	parent, err := c.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	hotstuffProposal := model.ProposalFromFlow(header, parent.View)
	err = c.validator.ValidateProposal(hotstuffProposal)
	if err != nil {
		if model.IsInvalidBlockError(err) {
			return engine.NewInvalidInputErrorf("invalid block proposal: %w", err)
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
			return engine.NewUnverifiableInputError("cannot validate proposal with view from unknown epoch: %w", err)
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
			return engine.NewInvalidInputErrorf("invalid extension of protocol state (block: %x, height: %d): %w", blockID, header.Height, err)
		}
		if state.IsOutdatedExtensionError(err) {
			// protocol state aborted processing of block as it is on an abandoned fork: block is outdated
			return engine.NewOutdatedInputErrorf("outdated extension of protocol state: %w", err)
		}

		// unexpected error: potentially corrupted internal state => abort processing and escalate error
		return fmt.Errorf("unexpected exception while extending protocol state with block %x at height %d: %w", blockID, header.Height, err)
	}

	// TODO replace with pubsub https://github.com/dapperlabs/flow-go/issues/6395
	c.hotstuff.SubmitProposal(header, parent.View)

	return nil
}

// OnBlockVote forwards incoming block votes to the `hotstuff.VoteAggregator`
// No errors are expected during normal operation.
func (c *Core) OnBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), vote.BlockID, trace.CONCompOnBlockVote)
	if isSampled {
		span.SetAttributes(
			attribute.String("origin_id", originID.String()),
		)
	}
	defer span.End()

	v := &model.Vote{
		View:     vote.View,
		BlockID:  vote.BlockID,
		SignerID: originID,
		SigData:  vote.SigData,
	}

	c.log.Info().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", v.SignerID[:]).
		Str("vote_id", v.ID().String()).
		Msg("block vote received, forwarding block vote to hotstuff vote aggregator")

	// forward the vote to hotstuff for processing
	c.voteAggregator.AddVote(v)

	return nil
}

// OnTimeoutObject forwards incoming TimeoutObjects to the `hotstuff.TimeoutAggregator`
// No errors are expected during normal operation.
func (c *Core) OnTimeoutObject(originID flow.Identifier, timeout *messages.TimeoutObject) error {
	t := &model.TimeoutObject{
		View:       timeout.View,
		NewestQC:   timeout.NewestQC,
		LastViewTC: timeout.LastViewTC,
		SignerID:   originID,
		SigData:    timeout.SigData,
	}

	c.log.Info().
		Hex("origin_id", originID[:]).
		Uint64("view", t.View).
		Str("timeout_id", t.ID().String()).
		Msg("timeout received, forwarding timeout to hotstuff timeout aggregator")

	// forward the timeout to hotstuff for processing
	c.timeoutAggregator.AddTimeout(t)

	return nil
}

// ProcessFinalizedView performs pruning of stale data based on finalization event
// removes pending blocks below the finalized view
func (c *Core) ProcessFinalizedView(finalizedView uint64) {
	// remove all pending blocks at or below the finalized view
	c.pending.PruneByView(finalizedView)

	// always record the metric
	c.mempoolMetrics.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
}
