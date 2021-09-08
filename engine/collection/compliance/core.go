// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"errors"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/onflow/flow-go/model/cluster"
	clusterkv "github.com/onflow/flow-go/state/cluster"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
// NOTE: Core is designed to be non-thread safe and cannot be used in concurrent environment
// user of this object needs to ensure single thread access.
type Core struct {
	log               zerolog.Logger // used to log relevant actions with context
	metrics           module.EngineMetrics
	tracer            module.Tracer
	mempoolMetrics    module.MempoolMetrics
	collectionMetrics module.CollectionMetrics
	cleaner           storage.Cleaner
	headers           storage.Headers
	payloads          storage.Payloads
	state             clusterkv.MutableState
	pending           module.PendingClusterBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
}

// NewCore creates a new consensus propagation engine.
func NewCore(
	log zerolog.Logger,
	collector module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	collectionMetrics module.CollectionMetrics,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state clusterkv.MutableState,
	pending module.PendingClusterBlockBuffer,
	sync module.BlockRequester,
) (*Core, error) {

	c := &Core{
		log:               log.With().Str("col_compliance", "core").Logger(),
		metrics:           collector,
		tracer:            tracer,
		mempoolMetrics:    mempool,
		collectionMetrics: collectionMetrics,
		cleaner:           cleaner,
		headers:           headers,
		payloads:          payloads,
		state:             state,
		pending:           pending,
		sync:              sync,
		hotstuff:          nil, // use `WithConsensus`
	}

	// log the mempool size off the bat
	c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

	return c, nil
}

// OnBlockProposal handles block proposals. Proposals are either processed
// immediately if possible, or added to the pending cache.
func (c *Core) OnBlockProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {
	header := proposal.Header
	payload := proposal.Payload

	log := c.log.With().
		Hex("origin_id", originID[:]).
		Hex("block_id", logging.Entity(header)).
		Uint64("block_height", header.Height).
		Str("chain_id", header.ChainID.String()).
		Hex("parent_id", logging.ID(header.ParentID)).
		Int("collection_size", payload.Collection.Len()).
		Logger()

	log.Debug().Msg("received proposal")

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

		c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

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

		// add the block to the cache
		_ = c.pending.Add(originID, proposal)

		c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		c.sync.RequestBlock(header.ParentID)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	err = c.processBlockProposal(proposal)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	return nil
}

// processBlockProposal processes a block that connects to finalized state.
// First we ensure the block is a valid extension of chain state, then store
// the block on disk, then enqueue the block for processing by HotStuff.
func (c *Core) processBlockProposal(proposal *messages.ClusterBlockProposal) error {

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

	log.Info().Msg("processing cluster block proposal")

	// extend the state with the proposal -- if it is an invalid extension,
	// we will throw an error here
	block := &cluster.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}

	err := c.state.Extend(block)
	// if the error is a known invalid extension of the cluster state, then
	// the input is invalid
	if state.IsInvalidExtensionError(err) {
		return engine.NewInvalidInputErrorf("invalid extension of cluster state: %w", err)
	}

	// if the error is an known outdated extension of the cluster state, then
	// the input is outdated
	if state.IsOutdatedExtensionError(err) {
		return engine.NewOutdatedInputErrorf("outdated extension of cluster state: %w", err)
	}

	if err != nil {
		return fmt.Errorf("could not extend cluster state: %w", err)
	}

	// retrieve the parent
	parent, err := c.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	log.Info().Msg("forwarding cluster block proposal to hotstuff")

	// submit the proposal to hotstuff for processing
	c.hotstuff.SubmitProposal(header, parent.View)

	// report proposed (case that we are not leader)
	c.collectionMetrics.ClusterBlockProposed(block)

	err = c.processPendingChildren(header)
	if err != nil {
		return fmt.Errorf("could not process pending children: %w", err)
	}

	return nil
}

// processPendingChildren handles processing pending children after successfully
// processing their parent. Regardless of whether processing succeeds, each
// child will be discarded (and re-requested later on if needed).
func (c *Core) processPendingChildren(header *flow.Header) error {
	blockID := header.ID()

	// check if there are any children for this parent in the cache
	children, ok := c.pending.ByParentID(blockID)
	if !ok {
		return nil
	}

	c.log.Debug().
		Int("children", len(children)).
		Msg("processing pending children")

	// then try to process children only this once
	result := new(multierror.Error)
	for _, child := range children {
		proposal := &messages.ClusterBlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := c.processBlockProposal(proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove children from cache
	c.pending.DropForParent(blockID)

	// flatten out the error tree before returning the error
	result = multierror.Flatten(result).(*multierror.Error)
	return result.ErrorOrNil()
}

// onBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (c *Core) onBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {

	c.log.Debug().
		Hex("origin_id", originID[:]).
		Hex("block_id", vote.BlockID[:]).
		Uint64("view", vote.View).
		Msg("received vote")

	c.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)
	return nil
}

// prunePendingCache prunes the pending block cache by removing any blocks that
// are below the finalized height.
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
	c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())
}
