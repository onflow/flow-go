// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
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
	con               network.Conduit
	pending           module.PendingBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
}

// NewCore creates a new consensus propagation engine.
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
) (*Core, error) {

	// initialize the propagation engine with its dependencies
	e := &Core{
		log:               log.With().Str("engine", "compliance").Logger(),
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
	}

	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	return e, nil
}

// OnSyncedBlock processes a block synced by the assembly engine.
func (c *Core) OnSyncedBlock(originID flow.Identifier, synced *events.SyncedBlock) error {
	// process as proposal
	proposal := &messages.BlockProposal{
		Header:  synced.Block.Header,
		Payload: synced.Block.Payload,
	}
	return c.onBlockProposal(synced.Block.Header.ProposerID, proposal)
}

// onBlockProposal handles incoming block proposals.
func (c *Core) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {
	blockSpan, ok := c.tracer.GetSpan(proposal.Header.ID(), trace.CONProcessBlock)
	if !ok {
		blockSpan = c.tracer.StartSpan(proposal.Header.ID(), trace.CONProcessBlock)
		blockSpan.SetTag("block_id", proposal.Header.ID())
		blockSpan.SetTag("view", proposal.Header.View)
		blockSpan.SetTag("proposer", proposal.Header.ProposerID.String())
	}
	onBlockProposalSpan := c.tracer.StartSpanFromParent(blockSpan, trace.CONCompOnBlockProposal)
	defer onBlockProposalSpan.Finish()

	for _, g := range proposal.Payload.Guarantees {
		if span, ok := c.tracer.GetSpan(g.CollectionID, trace.CONProcessCollection); ok {
			childSpan := c.tracer.StartSpanFromParent(span, trace.CONCompOnBlockProposal)
			defer childSpan.Finish()
		}
	}

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
	log.Info().Msg("block proposal received")

	c.prunePendingCache()

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
	recursiveProcessSpan := c.tracer.StartSpanFromParent(onBlockProposalSpan, trace.CONCompOnBlockProposalProcessRecursive)
	err = c.processBlockAndDescendants(proposal)
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
	recursiveProcessSpan.Finish()
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
	if err != nil {
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
			// child is outdated by the time we started processing it
			// => node was probably behind and is catching up. Log as warning
			if engine.IsOutdatedInputError(cpr) {
				c.log.Warn().Msg("dropped processing of abandoned fork; this might be an indicator that the node is slightly behind")
				continue
			}
			// the block is invalid; log as error as we desire honest participation
			// ToDo: potential slashing
			if engine.IsInvalidInputError(err) {
				c.log.Error().Err(err).Msg("invalid block (potential slashing evidence?)")
				continue
			}
			return fmt.Errorf("internal error processing block %x at height %d: %w", child.Header.ID(), child.Header.Height, err)
		}
	}

	// drop all of the children that should have been processed now
	c.pending.DropForParent(blockID)

	return nil
}

// processBlockProposal processes blocks that are already known to connect to
// the finalized state.
func (c *Core) processBlockProposal(proposal *messages.BlockProposal) error {
	startTime := time.Now()
	blockSpan, ok := c.tracer.GetSpan(proposal.Header.ID(), trace.CONProcessBlock)
	if !ok {
		blockSpan = c.tracer.StartSpan(proposal.Header.ID(), trace.CONProcessBlock)
		blockSpan.SetTag("block_id", proposal.Header.ID())
		blockSpan.SetTag("view", proposal.Header.View)
		blockSpan.SetTag("proposer", proposal.Header.ProposerID.String())
	}
	childSpan := c.tracer.StartSpanFromParent(blockSpan, trace.CONCompOnBlockProposalProcessSingle)
	defer func() {
		c.complianceMetrics.BlockProposalDuration(time.Since(startTime))
		childSpan.Finish()
	}()

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
	err := c.state.Extend(block)
	// if the block proposes an invalid extension of the protocol state, then the block is invalid
	if state.IsInvalidExtensionError(err) {
		return engine.NewInvalidInputErrorf("invalid extension of protocol state (block: %x, height: %d): %w",
			header.ID(), header.Height, err)
	}
	// of the protocol state aborted processing of block as it is on an abandoned fork, he is outdated
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

// onBlockVote handles incoming block votes.
func (c *Core) onBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	log := c.log.With().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", originID[:]).
		Logger()

	log.Info().Msg("block vote received")

	log.Info().Msg("forwarding block vote to hotstuff") // to keep logging consistent with proposals

	// forward the vote to hotstuff for processing
	c.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)

	return nil
}

// prunePendingCache prunes the pending block cache.
func (c *Core) prunePendingCache() {

	// retrieve the finalized height
	final, err := c.state.Final().Head()
	if err != nil {
		c.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
		return
	}

	// remove all pending blocks at or below the finalized height
	c.pending.PruneByHeight(final.Height)

	// always record the metric
	c.mempool.MempoolEntries(metrics.ResourceProposal, c.pending.Size())
}
