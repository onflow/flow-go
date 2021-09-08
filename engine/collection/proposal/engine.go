package compliance

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state"
	clusterkv "github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit           *engine.Unit
	log            zerolog.Logger
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	conduit        network.Conduit
	me             module.Local
	protoState     protocol.State         // flow-wide protocol chain state
	clusterState   clusterkv.MutableState // cluster-specific chain state
	transactions   storage.Transactions
	headers        storage.Headers
	payloads       storage.ClusterPayloads
	pending        module.PendingClusterBlockBuffer // pending block cache
	cluster        flow.IdentityList                // consensus participants in our cluster

	sync     module.BlockRequester
	hotstuff module.HotStuff
}

// New returns a new collection proposal engine.
func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	colMetrics module.CollectionMetrics,
	engMetrics module.EngineMetrics,
	mempoolMetrics module.MempoolMetrics,
	protoState protocol.State,
	clusterState clusterkv.MutableState,
	transactions storage.Transactions,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	cache module.PendingClusterBlockBuffer,
) (*Engine, error) {

	// find my cluster for the current epoch
	// TODO this should flow from cluster state as source of truth
	clusters, err := protoState.Final().Epochs().Current().Clustering()
	if err != nil {
		return nil, fmt.Errorf("could not get clusters: %w", err)
	}
	cluster, _, found := clusters.ByNodeID(me.NodeID())
	if !found {
		return nil, fmt.Errorf("could not find cluster for self")
	}

	// create the proposal engine
	e := &Engine{
		unit:           engine.NewUnit(),
		log:            log.With().Str("engine", "proposal").Logger(),
		colMetrics:     colMetrics,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		me:             me,
		protoState:     protoState,
		clusterState:   clusterState,
		transactions:   transactions,
		headers:        headers,
		payloads:       payloads,
		pending:        cache,
		cluster:        cluster,
		hotstuff:       nil, // must use WithHotStuff
		sync:           nil, // must use WithSync
	}

	chainID, err := clusterState.Params().ChainID()
	if err != nil {
		return nil, fmt.Errorf("could not get chain ID: %w", err)
	}

	// register network conduit
	conduit, err := net.Register(engine.ChannelConsensusCluster(chainID), e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.conduit = conduit

	// log the mempool size off the bat
	e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())

	return e, nil
}

func (e *Engine) WithHotStuff(hot module.HotStuff) *Engine {
	e.hotstuff = hot
	return e
}

func (e *Engine) WithSync(sync module.BlockRequester) *Engine {
	e.sync = sync
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff module")
	}
	if e.sync == nil {
		panic("must initialize compliance engine with synchronization module")
	}
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		err := e.conduit.Close()
		if err != nil {
			e.log.Error().Err(err).Msg("could not close conduit")
		}
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.process(e.me.NodeID(), event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

	log := e.log.With().
		Hex("collection_id", blockID[:]).
		Uint64("collection_view", view).
		Hex("recipient_id", recipientID[:]).
		Logger()
	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// TODO: this is a hot-fix to mitigate the effects of the following Unicast call blocking occasionally
	e.unit.Launch(func() {
		// send the vote the desired recipient
		err := e.conduit.Unicast(vote, recipientID)
		if err != nil {
			log.Warn().Err(err).Msg("could not send vote")
			return
		}
		e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockVote)
		log.Info().Msg("collection vote transmitted")
	})

	return nil
}

// BroadcastProposal submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// BroadcastProposalWithDelay submits a cluster block proposal (effectively a proposal
// for the next collection) to all the collection nodes in our cluster.
func (e *Engine) BroadcastProposalWithDelay(header *flow.Header, delay time.Duration) error {

	// first, check that we are the proposer of the block
	if header.ProposerID != e.me.NodeID() {
		return fmt.Errorf("cannot broadcast proposal with non-local proposer (%x)", header.ProposerID)
	}

	// get the parent of the block
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// fill in the fields that can't be populated by HotStuff
	//TODO clean this up - currently we set these fields in builder, then lose
	// them in HotStuff, then need to set them again here
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	log := e.log.With().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Logger()

	log.Debug().Msg("preparing to broadcast proposal from hotstuff")

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	log = log.With().Int("collection_size", payload.Collection.Len()).Logger()

	// retrieve all collection nodes in our cluster
	recipients, err := e.protoState.Final().Identities(filter.And(
		filter.In(e.cluster),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get cluster members: %w", err)
	}

	e.unit.LaunchAfter(delay, func() {

		go e.hotstuff.SubmitProposal(header, parent.View)

		// create the proposal message for the collection
		msg := &messages.ClusterBlockProposal{
			Header:  header,
			Payload: payload,
		}

		err := e.conduit.Publish(msg, recipients.NodeIDs()...)
		if errors.Is(err, network.EmptyTargetList) {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("could not broadcast proposal")
			return
		}

		log.Debug().
			Str("recipients", fmt.Sprintf("%v", recipients.NodeIDs())).
			Msg("broadcast proposal from hotstuff")

		e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockProposal)
		block := &cluster.Block{
			Header:  header,
			Payload: payload,
		}
		e.colMetrics.ClusterBlockProposed(block)
	})

	return nil
}

// process processes events for the proposal engine on the collection node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	// skip any message as long as we don't have the dependencies
	if e.hotstuff == nil || e.sync == nil {
		return fmt.Errorf("still initializing")
	}

	switch ev := event.(type) {
	case *events.SyncedClusterBlock:
		e.engMetrics.MessageReceived(metrics.EngineProposal, metrics.MessageSyncedClusterBlock)
		e.unit.Lock()
		defer e.engMetrics.MessageHandled(metrics.EngineProposal, metrics.MessageSyncedClusterBlock)
		defer e.unit.Unlock()
		return e.onSyncedBlock(originID, ev)
	case *messages.ClusterBlockProposal:
		e.engMetrics.MessageReceived(metrics.EngineProposal, metrics.MessageClusterBlockProposal)
		e.unit.Lock()
		defer e.engMetrics.MessageHandled(metrics.EngineProposal, metrics.MessageClusterBlockProposal)
		defer e.unit.Unlock()
		return e.onBlockProposal(originID, ev)
	case *messages.ClusterBlockVote:
		// we don't lock the engine on vote messages, because votes are passed
		// directly to HotStuff with no extra validation by compliance layer.
		e.engMetrics.MessageReceived(metrics.EngineProposal, metrics.MessageClusterBlockVote)
		defer e.engMetrics.MessageHandled(metrics.EngineProposal, metrics.MessageClusterBlockVote)
		return e.onBlockVote(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onSyncedBlock processes a block synced by the synchronization engine.
func (e *Engine) onSyncedBlock(originID flow.Identifier, synced *events.SyncedClusterBlock) error {

	// a block that is synced has to come locally, from the synchronization engine
	// the block itself will contain the proposer to indicate who created it
	if originID != e.me.NodeID() {
		return fmt.Errorf("synced block with non-local origin (local: %x, origin: %x)", e.me.NodeID(), originID)
	}

	// process as proposal
	proposal := &messages.ClusterBlockProposal{
		Header:  synced.Block.Header,
		Payload: synced.Block.Payload,
	}
	return e.onBlockProposal(synced.Block.Header.ProposerID, proposal)
}

// onBlockProposal handles block proposals. Proposals are either processed
// immediately if possible, or added to the pending cache.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.ClusterBlockProposal) error {

	header := proposal.Header
	payload := proposal.Payload

	log := e.log.With().
		Hex("origin_id", originID[:]).
		Hex("block_id", logging.Entity(header)).
		Uint64("block_height", header.Height).
		Str("chain_id", header.ChainID.String()).
		Hex("parent_id", logging.ID(header.ParentID)).
		Int("collection_size", payload.Collection.Len()).
		Logger()

	log.Debug().Msg("received proposal")

	e.prunePendingCache()

	// first, we reject all blocks that we don't need to process:
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization

	// ignore proposals that are already cached
	_, cached := e.pending.ByID(header.ID())
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// ignore proposals that were already processed
	_, err := e.headers.ByBlockID(header.ID())
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
	ancestor, found := e.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = e.pending.Add(originID, proposal)

		e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())

		// go to the first missing ancestor
		ancestorID := ancestor.Header.ParentID
		ancestorHeight := ancestor.Header.Height - 1
		for {
			ancestor, found := e.pending.ByID(ancestorID)
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

		e.sync.RequestBlock(ancestorID)

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = e.headers.ByBlockID(header.ParentID)
	if errors.Is(err, storage.ErrNotFound) {

		// add the block to the cache
		_ = e.pending.Add(originID, proposal)

		e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		e.sync.RequestBlock(header.ParentID)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	err = e.processBlockProposal(proposal)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	return nil
}

// processBlockProposal processes a block that connects to finalized state.
// First we ensure the block is a valid extension of chain state, then store
// the block on disk, then enqueue the block for processing by HotStuff.
func (e *Engine) processBlockProposal(proposal *messages.ClusterBlockProposal) error {

	header := proposal.Header

	log := e.log.With().
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

	err := e.clusterState.Extend(block)
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
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	log.Info().Msg("forwarding cluster block proposal to hotstuff")

	// submit the proposal to hotstuff for processing
	e.hotstuff.SubmitProposal(header, parent.View)

	// report proposed (case that we are not leader)
	e.colMetrics.ClusterBlockProposed(block)

	err = e.processPendingChildren(header)
	if err != nil {
		return fmt.Errorf("could not process pending children: %w", err)
	}

	return nil

}

// processPendingChildren handles processing pending children after successfully
// processing their parent. Regardless of whether processing succeeds, each
// child will be discarded (and re-requested later on if needed).
func (e *Engine) processPendingChildren(header *flow.Header) error {
	blockID := header.ID()

	// check if there are any children for this parent in the cache
	children, ok := e.pending.ByParentID(blockID)
	if !ok {
		return nil
	}

	e.log.Debug().
		Int("children", len(children)).
		Msg("processing pending children")

	// then try to process children only this once
	result := new(multierror.Error)
	for _, child := range children {
		proposal := &messages.ClusterBlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := e.processBlockProposal(proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove children from cache
	e.pending.DropForParent(blockID)

	// flatten out the error tree before returning the error
	result = multierror.Flatten(result).(*multierror.Error)
	return result.ErrorOrNil()
}

// onBlockVote handles votes for blocks by passing them to the core consensus
// algorithm
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.ClusterBlockVote) error {

	e.log.Debug().
		Hex("origin_id", originID[:]).
		Hex("block_id", vote.BlockID[:]).
		Uint64("view", vote.View).
		Msg("received vote")

	e.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)
	return nil
}

// prunePendingCache prunes the pending block cache by removing any blocks that
// are below the finalized height.
func (e *Engine) prunePendingCache() {

	// retrieve the finalized height
	final, err := e.clusterState.Final().Head()
	if err != nil {
		e.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
		return
	}

	// remove all pending blocks at or below the finalized height
	e.pending.PruneByHeight(final.Height)

	// always record the metric
	e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())
}
