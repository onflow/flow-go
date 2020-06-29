package proposal

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state"
	clusterkv "github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the collection proposal engine, which packages pending
// transactions into collections and sends them to consensus nodes.
type Engine struct {
	unit           *engine.Unit
	log            zerolog.Logger
	colMetrics     module.CollectionMetrics
	engMetrics     module.EngineMetrics
	mempoolMetrics module.MempoolMetrics
	con            network.Conduit
	me             module.Local
	protoState     protocol.State  // flow-wide protocol chain state
	clusterState   clusterkv.State // cluster-specific chain state
	validator      module.TransactionValidator
	pool           mempool.Transactions
	transactions   storage.Transactions
	headers        storage.Headers
	payloads       storage.ClusterPayloads
	pending        module.PendingClusterBlockBuffer // pending block cache
	participants   flow.IdentityList                // consensus participants in our cluster

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
	clusterState clusterkv.State,
	validator module.TransactionValidator,
	pool mempool.Transactions,
	transactions storage.Transactions,
	headers storage.Headers,
	payloads storage.ClusterPayloads,
	cache module.PendingClusterBlockBuffer,
	sync module.BlockRequester,
) (*Engine, error) {

	participants, err := protocol.ClusterFor(protoState.Final(), me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not get cluster participants: %w", err)
	}

	e := &Engine{
		unit:           engine.NewUnit(),
		log:            log.With().Str("engine", "proposal").Logger(),
		colMetrics:     colMetrics,
		engMetrics:     engMetrics,
		mempoolMetrics: mempoolMetrics,
		me:             me,
		protoState:     protoState,
		clusterState:   clusterState,
		validator:      validator,
		pool:           pool,
		transactions:   transactions,
		headers:        headers,
		payloads:       payloads,
		pending:        cache,
		participants:   participants,
		sync:           sync,
		hotstuff:       nil, // use WithHotStuff
	}

	e.mempoolMetrics.MempoolEntries(metrics.ResourceClusterProposal, e.pending.Size())

	con, err := net.Register(engine.ProtocolClusterConsensus, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// WithConsensus adds the consensus algorithm to the engine. This must be
// called before the engine can start.
func (e *Engine) WithConsensus(hot module.HotStuff) *Engine {
	e.hotstuff = hot
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.sync == nil {
		panic("must initialize compliance engine with synchronization module")
	}
	if e.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	return e.unit.Ready(func() {
		<-e.hotstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		e.log.Debug().Msg("shutting down hotstuff eventloop")
		<-e.hotstuff.Done()
		e.log.Debug().Msg("all components have been shut down")
	})
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// SendVote will send a vote to the desired node.
func (e *Engine) SendVote(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

	// build the vote message
	vote := &messages.ClusterBlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	err := e.con.Submit(vote, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	e.log.Debug().
		Hex("block_id", blockID[:]).
		Uint64("view", view).
		Hex("recipient_id", recipientID[:]).
		Msg("sending vote")

	e.engMetrics.MessageSent(metrics.EngineProposal, metrics.MessageClusterBlockVote)

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

	log := e.log.With().
		Hex("block_id", logging.ID(header.ID())).
		Uint64("block_height", header.Height).
		Logger()

	log.Debug().Msg("preparing to broadcast proposal from hotstuff")

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

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not get payload for block: %w", err)
	}

	log = log.With().Int("collection_size", payload.Collection.Len()).Logger()

	// retrieve all collection nodes in our cluster
	recipients, err := e.protoState.Final().Identities(filter.And(
		filter.In(e.participants),
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

		err = e.con.Submit(msg, recipients.NodeIDs()...)
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

	//TODO currently it is possible for network messages to be received
	// and passed to the engine before the engine has been setup
	// (ie had Ready called). This is a quickfix to get around the issue
	// but ultimately we should start receiving over the network only
	// once all the engines are ready.
	if e.sync == nil {
		return fmt.Errorf("missing sync dependency")
	}
	if e.hotstuff == nil {
		return fmt.Errorf("missing hotstuff dependency")
	}

	// just process one event at a time for now

	switch ev := event.(type) {
	case *events.SyncedClusterBlock:
		e.before(metrics.MessageSyncedClusterBlock)
		defer e.after(metrics.MessageSyncedClusterBlock)
		return e.onSyncedBlock(originID, ev)
	case *messages.ClusterBlockProposal:
		e.before(metrics.MessageClusterBlockProposal)
		defer e.after(metrics.MessageClusterBlockProposal)
		return e.onBlockProposal(originID, ev)
	case *messages.ClusterBlockVote:
		e.before(metrics.MessageClusterBlockVote)
		defer e.after(metrics.MessageClusterBlockVote)
		return e.onBlockVote(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) before(msg string) {
	e.engMetrics.MessageReceived(metrics.EngineProposal, msg)
	e.unit.Lock()
}

func (e *Engine) after(msg string) {
	e.unit.Unlock()
	e.engMetrics.MessageHandled(metrics.EngineProposal, msg)
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
	e.mempoolMetrics.MempoolEntries(metrics.ResourceTransaction, e.pool.Size())

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

	// we haven't seen this proposal yet, so at this point validate any
	// transactions we haven't yet seen
	var merr *multierror.Error
	for _, tx := range payload.Collection.Transactions {
		if !e.pool.Has(tx.ID()) {
			err = e.validator.ValidateTransaction(tx)
			if err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}
	if err := merr.ErrorOrNil(); err != nil {
		return engine.NewInvalidInputErrorf("cannot validate block proposal (id=%x) with invalid transactions: %w", header.ID(), err)
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

	err := e.clusterState.Mutate().Extend(block)
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
	var result *multierror.Error
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
