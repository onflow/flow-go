// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// max size of the pending cache, after which we prune blocks older than the
// finalized head.
const maxPending = 5000

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit       *engine.Unit   // used to control startup/shutdown
	log        zerolog.Logger // used to log relevant actions with context
	me         module.Local
	cleaner    storage.Cleaner
	headers    storage.Headers
	payloads   storage.Payloads
	state      protocol.State
	con        network.Conduit
	prov       network.Engine
	pending    module.PendingBlockBuffer // pending block cache
	maxPending uint                      // maximum size of the pending cache
	sync       module.Synchronization
	hotstuff   module.HotStuff
	metrics    module.Metrics
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.State,
	prov network.Engine,
	pending module.PendingBlockBuffer,
	metrics module.Metrics,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:       engine.NewUnit(),
		log:        log.With().Str("engine", "consensus").Logger(),
		me:         me,
		cleaner:    cleaner,
		headers:    headers,
		payloads:   payloads,
		state:      state,
		prov:       prov,
		pending:    pending,
		maxPending: maxPending,
		metrics:    metrics,
		sync:       nil, // use `WithSynchronization`
		hotstuff:   nil, // use `WithConsensus`
	}

	// register the engine with the network layer and store the conduit
	con, err := net.Register(engine.ProtocolConsensus, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}
	e.con = con

	return e, nil
}

// WithSynchronization adds the synchronization engine responsible for bringing the node
// up to speed to the compliance engine.
func (e *Engine) WithSynchronization(sync module.Synchronization) *Engine {
	e.sync = sync
	return e
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
	if e.sync == nil {
		panic("must initialize compliance engine with synchronization module")
	}
	if e.hotstuff == nil {
		panic("must initialize compliance engine with hotstuff engine")
	}
	return e.unit.Ready(func() {
		<-e.sync.Ready()
		<-e.hotstuff.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.sync.Done()
		<-e.hotstuff.Done()
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
			e.log.Error().Err(err).Msg("could not process submitted event")
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
		Uint64("block_view", view).
		Hex("block_id", blockID[:]).
		Hex("signer", logging.ID(e.me.NodeID())).
		Logger()

	log.Info().Msg("processing vote transmission request from hotstuff")

	// build the vote message
	vote := &messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sigData,
	}

	// send the vote the desired recipient
	err := e.con.Submit(vote, recipientID)
	if err != nil {
		return fmt.Errorf("could not send vote: %w", err)
	}

	log.Info().Msg("block vote transmitted")

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.
func (e *Engine) BroadcastProposal(header *flow.Header) error {

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

	log := e.log.With().
		Str("chain_id", header.ChainID).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("parent_voters", len(header.ParentVoterIDs)).
		Hex("parent_sig", header.ParentVoterSig[:]).
		Logger()

	log.Info().Msg("processing proposal broadcast request from hotstuff")

	// retrieve the payload for the block
	payload, err := e.payloads.ByBlockID(header.ID())
	if err != nil {
		return fmt.Errorf("could not retrieve payload for proposal: %w", err)
	}

	// retrieve all consensus nodes without our ID
	recipients, err := e.state.AtBlockID(header.ParentID).Identities(filter.And(
		filter.HasRole(flow.RoleConsensus),
		filter.Not(filter.HasNodeID(e.me.NodeID())),
	))
	if err != nil {
		return fmt.Errorf("could not get consensus recipients: %w", err)
	}

	// NOTE: some fields are not needed for the message
	// - proposer ID is conveyed over the network message
	// - the payload hash is deduced from the payload
	msg := &messages.BlockProposal{
		Header:  header,
		Payload: payload,
	}

	// broadcast the proposal to consensus nodes
	err = e.con.Submit(msg, recipients.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not send proposal message: %w", err)
	}

	log.Info().Msg("block proposal broadcasted")

	// submit the proposal to the provider engine to forward it to other
	// node roles
	e.prov.SubmitLocal(msg)

	// after broadcasting a new proposal is a great time to garbage collect
	// on badger; we won't need to do any heavy work soon, because the other
	// replicas will be busy validating our proposal
	e.cleaner.RunGC()

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, input interface{}) error {

	// process one event at a time for now
	e.unit.Lock()
	defer e.unit.Unlock()

	// skip any message as long as we don't have the dependencies
	if e.hotstuff == nil || e.sync == nil {
		return fmt.Errorf("still initializing")
	}

	switch v := input.(type) {
	case *events.SyncedBlock:
		return e.onSyncedBlock(originID, v)
	case *messages.BlockProposal:
		return e.onBlockProposal(originID, v)
	case *messages.BlockVote:
		return e.onBlockVote(originID, v)
	default:
		return fmt.Errorf("invalid event type (%T)", v)
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
		Header:  &synced.Block.Header,
		Payload: &synced.Block.Payload,
	}
	return e.onBlockProposal(synced.Block.ProposerID, proposal)
}

// onBlockProposal handles incoming block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	header := proposal.Header

	log := e.log.With().
		Str("chain_id", header.ChainID).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("parent_voters", len(header.ParentVoterIDs)).
		Hex("parent_sig", header.ParentVoterSig[:]).
		Logger()

	log.Info().Msg("block proposal received")

	if e.pending.Size() > e.maxPending {
		e.prunePendingCache()
	}
	e.metrics.PendingBlocks(e.pending.Size())

	// first, we reject all blocks that we don't need to process:
	// 1) blocks already in the cache; they will already be processed later
	// 2) blocks already on disk; they were processed and await finalization
	// 3) blocks at a height below finalized height; they can not be finalized

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
		return fmt.Errorf("could not retrieve header: %w", err)
	}

	// get the latest finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get final block: %w", err)
	}

	// ignore proposals that can no longer become valid
	if header.Height <= final.Height {
		log.Debug().Msg("skipping already outdated proposal")
		return nil
	}

	// from here on out, there are two possibilities:
	// 1) the proposal is unverifiable because parent or ancestor is unknown
	// => in each case, we cache the proposal and request the missing link
	// 2) the proposal is connected to finalized state through an unbroken chain
	// => we verify the proposal and forward it to hotstuff if valid

	// if we can connect the proposal to an ancestor in the cache, it means
	// there is a missing link; we cache it and request the missing link
	ancestor, found := e.pending.ByID(header.ParentID)
	if found {

		// add the block to the cache
		_ = e.pending.Add(originID, proposal)

		// go to the first missing ancestor
		ancestorID := ancestor.Header.ParentID
		for {
			ancestor, found := e.pending.ByID(ancestorID)
			if !found {
				break
			}
			ancestorID = ancestor.Header.ParentID
		}

		log.Debug().Msg("requesting missing ancestor for proposal")

		e.sync.RequestBlock(ancestorID)

		return nil
	}

	// if the proposal is connected to a block that is neither in the cache, nor
	// in persistent storage, its direct parent is missing; cache the proposal
	// and request the parent
	_, err = e.headers.ByBlockID(header.ParentID)
	if err == storage.ErrNotFound {

		_ = e.pending.Add(originID, proposal)

		log.Debug().Msg("requesting missing parent for proposal")

		e.sync.RequestBlock(header.ParentID)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get parent: %w", err)
	}

	// at this point, we should be able to connect the proposal to the finalized
	// state and should process it to see whether to forward to hotstuff or not
	err = e.processBlockProposal(proposal)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	return nil
}

// processBlockProposal processes blocks that are already known to connect to
// the finalized state; if a parent of children is validly processed, it means
// the children are also still on a valid chain and all missing links are there;
// no need to do all the processing again.
func (e *Engine) processBlockProposal(proposal *messages.BlockProposal) error {

	header := proposal.Header

	log := e.log.With().
		Str("chain_id", header.ChainID).
		Uint64("block_height", header.Height).
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Hex("parent_id", header.ParentID[:]).
		Hex("payload_hash", header.PayloadHash[:]).
		Time("timestamp", header.Timestamp).
		Hex("proposer", header.ProposerID[:]).
		Int("parent_voters", len(header.ParentVoterIDs)).
		Hex("parent_sig", header.ParentVoterSig[:]).
		Logger()

	log.Info().Msg("processing block proposal")

	// store the proposal block payload
	err := e.payloads.Store(header, proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store proposal payload: %w", err)
	}

	// store the proposal block header
	err = e.headers.Store(header)
	if err != nil {
		return fmt.Errorf("could not store proposal header: %w", err)
	}

	// see if the block is a valid extension of the protocol state
	err = e.state.Mutate().Extend(header.ID())
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
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

	// check if there are any children for this parent in the cache
	children, has := e.pending.ByParentID(header.ID())
	if !has {
		return nil
	}

	// then try to process children only this once
	var result *multierror.Error
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
	e.pending.DropForParent(header)

	return result.ErrorOrNil()
}

// prunePendingCache prunes the pending block cache.
func (e *Engine) prunePendingCache() {
	// retrieve the finalized height
	final, err := e.state.Final().Head()
	if err != nil {
		e.log.Warn().Err(err).Msg("could not get finalized head to prune pending blocks")
	}
	e.pending.PruneByHeight(final.Height)
}
