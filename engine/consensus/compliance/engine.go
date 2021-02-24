// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
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

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit     *engine.Unit   // used to control startup/shutdown
	log      zerolog.Logger // used to log relevant actions with context
	metrics  module.EngineMetrics
	tracer   module.Tracer
	mempool  module.MempoolMetrics
	spans    module.ConsensusMetrics
	me       module.Local
	cleaner  storage.Cleaner
	headers  storage.Headers
	payloads storage.Payloads
	state    protocol.MutableState
	con      network.Conduit
	prov     network.Engine
	pending  module.PendingBlockBuffer // pending block cache
	sync     module.BlockRequester
	hotstuff module.HotStuff
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
	collector module.EngineMetrics,
	tracer module.Tracer,
	mempool module.MempoolMetrics,
	spans module.ConsensusMetrics,
	net module.Network,
	me module.Local,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.MutableState,
	prov network.Engine,
	pending module.PendingBlockBuffer,
	sync module.BlockRequester,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "compliance").Logger(),
		metrics:  collector,
		tracer:   tracer,
		mempool:  mempool,
		spans:    spans,
		me:       me,
		cleaner:  cleaner,
		headers:  headers,
		payloads: payloads,
		state:    state,
		prov:     prov,
		pending:  pending,
		sync:     sync,
		hotstuff: nil, // use `WithConsensus`
	}

	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ConsensusCommittee, e)
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
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	return e.unit.Ready(func() {
		<-e.hotstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
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
		err := e.Process(originID, event)
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

	log := e.log.With().
		Hex("block_id", blockID[:]).
		Uint64("block_view", view).
		Hex("recipient_id", recipientID[:]).
		Logger()
	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// TODO: this is a hot-fix to mitigate the effects of the following Unicast call blocking occasionally
	e.unit.Launch(func() {
		// send the vote the desired recipient
		err := e.con.Unicast(vote, recipientID)
		if err != nil {
			log.Warn().Err(err).Msg("could not send vote")
			return
		}
		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockVote)
		log.Info().Msg("block vote transmitted")
	})

	return nil
}

// BroadcastProposalWithDelay will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
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
	header.ChainID = parent.ChainID
	header.Height = parent.Height + 1

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	log := e.log.With().
		Str("chain_id", header.ChainID.String()).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Int("gaurantees_count", len(payload.Guarantees)).
		Int("seals_count", len(payload.Seals)).
		Int("receipts_count", len(payload.Receipts)).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("num_signers", len(header.ParentVoterIDs)).
		Dur("delay", delay).
		Logger()

	log.Debug().Msg("processing proposal broadcast request from hotstuff")

	for _, g := range payload.Guarantees {
		if span, ok := e.tracer.GetSpan(g.CollectionID, trace.CONProcessCollection); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONCompBroadcastProposalWithDelay)
			defer childSpan.Finish()
		}
	}

	// retrieve all consensus nodes without our ID
	recipients, err := e.state.AtBlockID(header.ParentID).Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get consensus recipients: %w", err)
	}

	e.unit.LaunchAfter(delay, func() {

		go e.hotstuff.SubmitProposal(header, parent.View)

		// NOTE: some fields are not needed for the message
		// - proposer ID is conveyed over the network message
		// - the payload hash is deduced from the payload
		proposal := &messages.BlockProposal{
			Header:  header,
			Payload: payload,
		}

		// broadcast the proposal to consensus nodes
		err = e.con.Publish(proposal, recipients.NodeIDs()...)
		if err != nil {
			log.Error().Err(err).Msg("could not send proposal message")
		}

		e.metrics.MessageSent(metrics.EngineCompliance, metrics.MessageBlockProposal)

		log.Info().Msg("block proposal broadcasted")

		// submit the proposal to the provider engine to forward it to other
		// node roles
		e.prov.SubmitLocal(proposal)
	})

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
func (e *Engine) BroadcastProposal(header *flow.Header) error {
	return e.BroadcastProposalWithDelay(header, 0)
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {

	// skip any message as long as we don't have the dependencies
	if e.hotstuff == nil || e.sync == nil {
		return fmt.Errorf("still initializing")
	}

	switch ev := event.(type) {
	case *events.SyncedBlock:
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageSyncedBlock)
		e.unit.Lock()
		defer e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageSyncedBlock)
		defer e.unit.Unlock()
		return e.onSyncedBlock(originID, ev)
	case *messages.BlockProposal:
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockProposal)
		e.unit.Lock()
		defer e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageBlockProposal)
		defer e.unit.Unlock()
		return e.onBlockProposal(originID, ev)
	case *messages.BlockVote:
		// we don't lock the engine on vote messages, because votes are passed
		// directly to HotStuff with no extra validation by compliance layer.
		e.metrics.MessageReceived(metrics.EngineCompliance, metrics.MessageBlockVote)
		defer e.metrics.MessageHandled(metrics.EngineCompliance, metrics.MessageBlockVote)
		return e.onBlockVote(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onSyncedBlock processes a block synced by the assembly engine.
func (e *Engine) onSyncedBlock(originID flow.Identifier, synced *events.SyncedBlock) error {

	// a block that is synced has to come locally, from the synchronization engine
	// the block itself will contain the proposer to indicate who created it
	if originID != e.me.NodeID() {
		return fmt.Errorf("synced block with non-local origin (local: %x, origin: %x)", e.me.NodeID(), originID)
	}

	// process as proposal
	proposal := &messages.BlockProposal{
		Header:  synced.Block.Header,
		Payload: synced.Block.Payload,
	}
	return e.onBlockProposal(synced.Block.Header.ProposerID, proposal)
}

// onBlockProposal handles incoming block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	span, ok := e.tracer.GetSpan(proposal.Header.ID(), trace.CONProcessBlock)
	if !ok {
		span = e.tracer.StartSpan(proposal.Header.ID(), trace.CONProcessBlock)
		span.SetTag("block_id", proposal.Header.ID())
		span.SetTag("view", proposal.Header.View)
		span.SetTag("proposer", proposal.Header.ProposerID.String())
	}
	childSpan := e.tracer.StartSpanFromParent(span, trace.CONCompOnBlockProposal)
	defer childSpan.Finish()

	for _, g := range proposal.Payload.Guarantees {
		if span, ok := e.tracer.GetSpan(g.CollectionID, trace.CONProcessCollection); ok {
			childSpan := e.tracer.StartSpanFromParent(span, trace.CONCompOnBlockProposal)
			defer childSpan.Finish()
		}
	}

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

	log.Info().Msg("block proposal received")

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

		e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

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

		_ = e.pending.Add(originID, proposal)

		e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

		log.Debug().Msg("requesting missing parent for proposal")

		e.sync.RequestBlock(header.ParentID)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not check parent: %w", err)
	}

	// at this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not
	err = e.processBlockProposal(proposal)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	// most of the heavy database checks are done at this point, so this is a
	// good moment to potentially kick-off a garbage collection of the DB
	// NOTE: this is only effectively run every 1000th calls, which corresponds
	// to every 1000th successfully processed block
	e.cleaner.RunGC()

	return nil
}

// processBlockProposal processes blocks that are already known to connect to
// the finalized state; if a parent of children is validly processed, it means
// the children are also still on a valid chain and all missing links are there;
// no need to do all the processing again.
func (e *Engine) processBlockProposal(proposal *messages.BlockProposal) error {

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

	log.Info().Msg("processing block proposal")

	// see if the block is a valid extension of the protocol state
	block := &flow.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}

	err := e.state.Extend(block)
	// if the error is a known invalid extension of the protocol state, then
	// the input is invalid
	if state.IsInvalidExtensionError(err) {
		return engine.NewInvalidInputErrorf("invalid extension of protocol state (block: %x, height: %d): %w",
			header.ID(), header.Height, err)
	}

	// if the error is an known outdated extension of the protocol state, then
	// the input is outdated
	if state.IsOutdatedExtensionError(err) {
		return engine.NewOutdatedInputErrorf("outdated extension of protocol state: %w", err)
	}

	if err != nil {
		return fmt.Errorf("could not extend protocol state (block: %x, height: %d): %w", header.ID(), header.Height, err)
	}

	// retrieve the parent
	parent, err := e.headers.ByBlockID(header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	log.Info().Msg("forwarding block proposal to hotstuff")

	// submit the model to hotstuff for processing
	e.hotstuff.SubmitProposal(header, parent.View)

	// check for any descendants of the block to process
	err = e.processPendingChildren(header)
	if err != nil {
		return fmt.Errorf("could not process pending children: %w", err)
	}

	return nil
}

// onBlockVote handles incoming block votes.
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	log := e.log.With().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("voter", originID[:]).
		Logger()

	log.Info().Msg("block vote received")

	log.Info().Msg("forwarding block vote to hotstuff") // to keep logging consistent with proposals

	// forward the vote to hotstuff for processing
	e.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)

	return nil
}

// processPendingChildren checks if there are proposals connected to the given
// parent block that was just processed; if this is the case, they should now
// all be validly connected to the finalized state and we should process them.
func (e *Engine) processPendingChildren(header *flow.Header) error {
	blockID := header.ID()

	// check if there are any children for this parent in the cache
	children, has := e.pending.ByParentID(blockID)
	if !has {
		return nil
	}

	e.log.Debug().
		Int("children", len(children)).
		Msg("processing pending children")

	// then try to process children only this once
	result := new(multierror.Error)
	for _, child := range children {
		proposal := &messages.BlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := e.processBlockProposal(proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// drop all of the children that should have been processed now
	e.pending.DropForParent(blockID)

	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())

	// flatten out the error tree before returning the error
	result = multierror.Flatten(result).(*multierror.Error)
	return result.ErrorOrNil()
}

// prunePendingCache prunes the pending block cache.
func (e *Engine) prunePendingCache() {

	// retrieve the finalized height
	final, err := e.state.Final().Head()
	if err != nil {
		e.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
		return
	}

	// remove all pending blocks at or below the finalized height
	e.pending.PruneByHeight(final.Height)

	// always record the metric
	e.mempool.MempoolEntries(metrics.ResourceProposal, e.pending.Size())
}
