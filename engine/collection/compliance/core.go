// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state"
	clusterkv "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Core contains the central business logic for the collector clusters' compliance engine.
// It is responsible for handling communication for the embedded consensus algorithm.
// NOTE: Core is designed to be non-thread safe and cannot be used in concurrent environment
// user of this object needs to ensure single thread access.
type Core struct {
	log               zerolog.Logger // used to log relevant actions with context
	config            compliance.Config
	engineMetrics     module.EngineMetrics
	mempoolMetrics    module.MempoolMetrics
	collectionMetrics module.CollectionMetrics
	headers           storage.Headers
	state             clusterkv.MutableState
	pending           module.PendingClusterBlockBuffer // pending block cache
	sync              module.BlockRequester
	hotstuff          module.HotStuff
	validator         hotstuff.Validator
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
}

// NewCore instantiates the business logic for the collector clusters' compliance engine.
func NewCore(
	log zerolog.Logger,
	collector module.EngineMetrics,
	mempool module.MempoolMetrics,
	collectionMetrics module.CollectionMetrics,
	headers storage.Headers,
	state clusterkv.MutableState,
	pending module.PendingClusterBlockBuffer,
	validator hotstuff.Validator,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	opts ...compliance.Opt,
) (*Core, error) {

	config := compliance.DefaultConfig()
	for _, apply := range opts {
		apply(&config)
	}

	c := &Core{
		log:               log.With().Str("cluster_compliance", "core").Logger(),
		config:            config,
		engineMetrics:     collector,
		mempoolMetrics:    mempool,
		collectionMetrics: collectionMetrics,
		headers:           headers,
		state:             state,
		pending:           pending,
		sync:              nil, // use `WithSync`
		hotstuff:          nil, // use `WithConsensus`
		validator:         validator,
		voteAggregator:    voteAggregator,
		timeoutAggregator: timeoutAggregator,
	}

	// log the mempool size off the bat
	c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

	return c, nil
}

// OnBlockProposal handles incoming block proposals.
func (c *Core) OnBlockProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {
	header := proposal.Header
	blockID := header.ID()
	log := c.log.With().
		Hex("origin_id", originID[:]).
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", blockID[:]).
		Hex("parent_id", header.ParentID[:]).
		Hex("ref_block_id", proposal.Payload.ReferenceBlockID[:]).
		Hex("collection_id", logging.Entity(proposal.Payload.Collection)).
		Int("tx_count", proposal.Payload.Collection.Len()).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Hex("signers", header.ParentVoterIndices).
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
		c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = c.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		_ = c.pending.Add(originID, proposal)

		c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		c.sync.RequestBlock(header.ParentID, header.Height-1)

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
	c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	return nil
}

// processBlockAndDescendants is a recursive function that processes a block and
// its pending proposals for its children. By induction, any children connected
// to a valid proposal are validly connected to the finalized state and can be
// processed as well.
func (c *Core) processBlockAndDescendants(proposal *messages.ClusterBlockProposal) error {
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
		childProposal := &messages.ClusterBlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		cpr := c.processBlockAndDescendants(childProposal)
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
//   - engine.InvalidInputError if the block proposal is invalid
//   - engine.UnverifiableInputError if the proposal cannot be validated
func (c *Core) processBlockProposal(proposal *messages.ClusterBlockProposal) error {
	header := proposal.Header
	blockID := header.ID()
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

	// see if the block is a valid extension of the protocol state
	block := &cluster.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}
	err = c.state.Extend(block)
	if err != nil {
		if state.IsInvalidExtensionError(err) {
			// if the block proposes an invalid extension of the cluster state, then the block is invalid
			return engine.NewInvalidInputErrorf("invalid extension of cluster state (block: %x, height: %d): %w", blockID, header.Height, err)
		}
		if state.IsOutdatedExtensionError(err) {
			// cluster state aborted processing of block as it is on an abandoned fork: block is outdated
			return engine.NewOutdatedInputErrorf("outdated extension of cluster state: %w", err)
		}
		// unexpected error: potentially corrupted internal state => abort processing and escalate error
		return fmt.Errorf("unexpected exception while extending cluster state with block %x at height %d: %w", blockID, header.Height, err)
	}

	// submit the model to hotstuff for processing
	// TODO replace with pubsub https://github.com/dapperlabs/flow-go/issues/6395
	log.Info().Msg("forwarding block proposal to hotstuff")
	c.hotstuff.SubmitProposal(header, parent.View)

	return nil
}

// OnBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (c *Core) OnBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {

	c.log.Debug().
		Hex("origin_id", originID[:]).
		Hex("block_id", vote.BlockID[:]).
		Uint64("view", vote.View).
		Msg("received vote")

	c.voteAggregator.AddVote(&model.Vote{
		View:     vote.View,
		BlockID:  vote.BlockID,
		SignerID: originID,
		SigData:  vote.SigData,
	})
	return nil
}

func (c *Core) OnTimeoutObject(originID flow.Identifier, timeout *messages.ClusterTimeoutObject) error {
	t := &model.TimeoutObject{
		View:       timeout.View,
		NewestQC:   timeout.NewestQC,
		LastViewTC: timeout.LastViewTC,
		SignerID:   originID,
		SigData:    timeout.SigData,
	}

	c.log.Debug().
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
	c.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, c.pending.Size())
}
