// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"
	"github.com/uber/jaeger-client-go"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
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
	metrics           module.EngineMetrics
	tracer            module.Tracer
	mempool           module.MempoolMetrics
	complianceMetrics module.ComplianceMetrics
	cleaner           storage.Cleaner
	headers           storage.Headers
	payloads          storage.Payloads
	state             protocol.MutableState
	pending           module.PendingBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
	voteAggregator    hotstuff.VoteAggregator
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
	voteAggregator hotstuff.VoteAggregator,
) (*Core, error) {

	e := &Core{
		log:               log.With().Str("compliance", "core").Logger(),
		metrics:           collector,
		tracer:            tracer,
		mempool:           mempool,
		complianceMetrics: complianceMetrics,
		cleaner:           cleaner,
		headers:           headers,
		payloads:          payloads,
		state:             state,
		pending:           pending,
		sync:              sync,
		hotstuff:          nil, // use `WithConsensus`
		voteAggregator:    voteAggregator,
	}

	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	return e, nil
}

// OnBlockProposal handles incoming block proposals.
func (c *Core) OnBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	var traceID string

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), proposal.Header.ID(), trace.CONCompOnBlockProposal)
	if isSampled {
		span.LogFields(log.Uint64("view", proposal.Header.View))
		span.LogFields(log.String("origin_id", originID.String()))

		// set proposer as a tag so we can filter based on proposer
		span.SetTag("proposer", proposal.Header.ProposerID.String())
		if sc, ok := span.Context().(jaeger.SpanContext); ok {
			traceID = sc.TraceID().String()
		}
	}
	defer span.Finish()

	header := proposal.Header
	log := c.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("num_signers", len(header.ParentVoterIDs)).
		Str("traceID", traceID). // traceID is used to connect logs to traces
		Logger()
	log.Info().Msg("block proposal received")

	// first, we reject all blocks that we don't need to process:
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization

	// ignore proposals that are already cached
	_, cached := c.pending.ByID(header.ID())
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := c.headers.ByBlockID(header.ID())
	if err == nil {
		log.Debug().Msg("skipping already processed proposal")
		return nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not check proposal: %w", err)
	}

	// there are two possibilities if the proposal is neither already pending
	// processing in the cache, nor has already been processed:
	// 1) the proposal is unverifiable because parent or ancestor is unknown
	// => we cache the proposal and request the missing link
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if we can connect the proposal to an ancestor in the cache, it means
	// there is a missing link; we cache it and request the missing link
	ancestor, found := c.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = c.pending.Add(originID, proposal)
		c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		// go to the first missing ancestor
		ancestorID := ancestor.Header.ParentID
		ancestorHeight := ancestor.Header.Height - 1
		for {
			ancestor, found := c.pending.ByID(ancestorID)
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

		c.sync.RequestBlock(ancestorID)

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = c.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		_ = c.pending.Add(originID, proposal)

		c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		c.sync.RequestBlock(header.ParentID)

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
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
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
func (c *Core) processBlockAndDescendants(proposal *messages.BlockProposal) error {
	blockID := proposal.Header.ID()

	// process block itself
	err := c.processBlockProposal(proposal)
	// child is outdated by the time we started processing it
	// => node was probably behind and is catching up. Log as warning
	if engine.IsOutdatedInputError(err) {
		c.log.Info().Msg("dropped processing of abandoned fork; this might be an indicator that the node is slightly behind")
		return nil
	}
	// the block is invalid; log as error as we desire honest participation
	// ToDo: potential slashing
	if engine.IsInvalidInputError(err) {
		c.log.Warn().Err(err).Msg("received invalid block from other node (potential slashing evidence?)")
		return nil
	}
	if err != nil {
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
func (c *Core) processBlockProposal(proposal *messages.BlockProposal) error {
	startTime := time.Now()
	defer c.complianceMetrics.BlockProposalDuration(time.Since(startTime))

	span, ctx, isSampled := c.tracer.StartBlockSpan(context.Background(), proposal.Header.ID(), trace.ConCompProcessBlockProposal)
	if isSampled {
		span.SetTag("proposer", proposal.Header.ProposerID.String())
	}
	defer span.Finish()

	header := proposal.Header
	log := c.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("num_signers", len(header.ParentVoterIDs)).
		Logger()
	log.Info().Msg("processing block proposal")

	// see if the block is a valid extension of the protocol state
	block := &flow.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}
	err := c.state.Extend(ctx, block)
	// if the block proposes an invalid extension of the protocol state, then the block is invalid
	if state.IsInvalidExtensionError(err) {
		return engine.NewInvalidInputErrorf("invalid extension of protocol state (block: %x, height: %d): %w",
			header.ID(), header.Height, err)
	}
	// protocol state aborted processing of block as it is on an abandoned fork: block is outdated
	if state.IsOutdatedExtensionError(err) {
		return engine.NewOutdatedInputErrorf("outdated extension of protocol state: %w", err)
	}
	if err != nil {
		return fmt.Errorf("could not extend protocol state (block: %x, height: %d): %w", header.ID(), header.Height, err)
	}

	// retrieve the parent
	parent, err := c.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// submit the model to hotstuff for processing
	log.Info().Msg("forwarding block proposal to hotstuff")
	c.hotstuff.SubmitProposal(header, parent.View)

	return nil
}

// OnBlockVote handles incoming block votes.
func (c *Core) OnBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	span, _, isSampled := c.tracer.StartBlockSpan(context.Background(), vote.BlockID, trace.CONCompOnBlockVote)
	if isSampled {
		span.LogFields(log.String("origin_id", originID.String()))
	}
	defer span.Finish()

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

// ProcessFinalizedView performs pruning of stale data based on finalization event
// removes pending blocks below the finalized view
func (c *Core) ProcessFinalizedView(finalizedView uint64) {
	// remove all pending blocks at or below the finalized view
	c.pending.PruneByView(finalizedView)

	// always record the metric
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
}
