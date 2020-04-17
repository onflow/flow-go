// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package compliance

import (
	"fmt"
	"sync"

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

// Engine is the consensus engine, responsible for handling communication for
// the embedded consensus algorithm.
type Engine struct {
	unit       *engine.Unit   // used to control startup/shutdown
	log        zerolog.Logger // used to log relevant actions with context
	me         module.Local
	state      protocol.State
	headers    storage.Headers
	payloads   storage.Payloads
	con        network.Conduit
	buffer     module.PendingBlockBuffer
	cache      map[flow.Identifier]*flow.Header
	prov       network.Engine
	sync       module.Synchronization
	hotstuff   module.HotStuff
	sync.Mutex // temporary fix for pending cache concurrent access
}

// New creates a new consensus propagation engine.
func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	headers storage.Headers,
	payloads storage.Payloads,
	buffer module.PendingBlockBuffer,
	prov network.Engine,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "consensus").Logger(),
		me:       me,
		state:    state,
		headers:  headers,
		payloads: payloads,
		buffer:   buffer,
		cache:    make(map[flow.Identifier]*flow.Header),
		prov:     prov,
		sync:     nil, // use `WithSynchronization`
		hotstuff: nil, // use `WithConsensus`
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

	e.log.Info().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("recipient", recipientID[:]).
		Msg("block vote transmitted")

	return nil
}

// BroadcastProposal will propagate a block proposal to all non-local consensus nodes.
// Note the header has incomplete fields, because it was converted from a hotstuff.Proposal type
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

	// retrieve the payload for the block
	blockID := header.ID()
	payload, err := e.payloads.ByBlockID(blockID)
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

	e.log.Info().
		Uint64("block_view", header.View).
		Hex("block_id", logging.Entity(header)).
		Interface("recipients", recipients.NodeIDs()).
		Msg("block proposal broadcasted")

	// let the provider engine broadcast to the rest of the network
	e.prov.SubmitLocal(msg)

	return nil
}

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, input interface{}) error {
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
	return e.onBlockProposal(originID, proposal)
}

// onBlockProposal handles incoming block proposals; it checks whether we actually need
// to process them before forwarding to processing.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {
	e.Lock()
	defer e.Unlock()

	blockID := proposal.Header.ID()
	log := e.log.With().
		Uint64("block_view", proposal.Header.View).
		Hex("block_id", blockID[:]).
		Hex("sender", originID[:]).
		Logger()

	log.Info().Msg("block proposal received")

	// if we already have the proposal cached, no need to go again
	_, cached := e.cache[blockID]
	if cached {
		log.Info().Msg("skipping cached proposal")
		return nil
	}

	// get the latest finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get final block: %w", err)
	}

	// reject orphaned block
	if proposal.Header.Height <= final.Height {
		e.log.Info().Msg("skipping orphaned proposal")
		return nil
	}

	// TODO: turn cache into module and clean up upon block finalization

	// we should store the proposal in our pending cache
	e.cache[proposal.Header.ID()] = proposal.Header

	// check if the block is connected to the current finalized state
	finalID := final.ID()
	ancestorID := proposal.Header.ParentID
	for ancestorID != finalID {
		ancestor, ok := e.cache[ancestorID]
		if !ok {
			return e.handleMissingAncestor(originID, proposal, ancestorID)
		}
		ancestorID = ancestor.ParentID
	}

	// process the proposal
	err = e.processProposal(originID, proposal)
	if err != nil {
		return fmt.Errorf("could not process proposal: %w", err)
	}

	// for now, we clean up by finalization height bruteforce
	for cachedID, cached := range e.cache {
		if cached.Height <= final.Height {
			delete(e.cache, cachedID)
		}
	}

	return nil
}

func (e *Engine) processProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	blockID := proposal.Header.ID()

	// store the proposal block payload
	err := e.payloads.Store(proposal.Header, proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store payload (height: %d, view: %d, block_id: %x): %w",
			proposal.Header.Height, proposal.Header.View, blockID, err)
	}

	// store the proposal block header
	err = e.headers.Store(proposal.Header)
	if err != nil {
		return fmt.Errorf("could not store header (height: %d, view: %d, block_id: %x): %w",
			proposal.Header.Height, proposal.Header.View, blockID, err)
	}

	// see if the block is a valid extension of the protocol state
	err = e.state.Mutate().Extend(blockID)
	if err != nil {
		return fmt.Errorf("could not extend protocol state (height: %d, view: %d, block_id: %x): %w",
			proposal.Header.Height, proposal.Header.View, blockID, err)
	}

	// retrieve the parent
	parent, err := e.headers.ByBlockID(proposal.Header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	// submit the model to hotstuff for processing
	e.hotstuff.SubmitProposal(proposal.Header, parent.View)

	// check for any descendants of the block to process
	err = e.handleConnectedChildren(blockID)
	if err != nil {
		return fmt.Errorf("could not process proposal children: %w", err)
	}

	return nil
}

// onBlockVote handles incoming block votes.
func (e *Engine) onBlockVote(originID flow.Identifier, vote *messages.BlockVote) error {

	e.log.Info().
		Uint64("block_view", vote.View).
		Hex("block_id", vote.BlockID[:]).
		Hex("sender", originID[:]).
		Msg("block vote received")

	// forward the vote to hotstuff for processing
	e.hotstuff.SubmitVote(originID, vote.BlockID, vote.View, vote.SigData)

	return nil
}

// handleMissingAncestor will deal with proposals where one of the parents
// between proposal and finalized state is missing.
func (e *Engine) handleMissingAncestor(originID flow.Identifier, proposal *messages.BlockProposal, ancestorID flow.Identifier) error {

	// add the block to the cache for later processing; if it's already in the
	// cache, we are done
	pendingBlock := &flow.PendingBlock{
		OriginID: originID,
		Header:   proposal.Header,
		Payload:  proposal.Payload,
	}
	added := e.buffer.Add(pendingBlock)
	if !added {
		return nil
	}

	// add the block to the downlod queue
	e.sync.RequestBlock(ancestorID)

	return nil
}

// handleConnectedChildren checks if the given block has connected some children
// that were missing a link to the finalized state, in order to process them as
// well.
func (e *Engine) handleConnectedChildren(parentID flow.Identifier) error {

	// check if there are any children for this parent in the cache
	children, ok := e.buffer.ByParentID(parentID)
	if !ok {
		return nil
	}

	// then try to process children only this once
	var result *multierror.Error
	for _, child := range children {
		proposal := &messages.BlockProposal{
			Header:  child.Header,
			Payload: child.Payload,
		}
		err := e.processProposal(child.OriginID, proposal)
		if err != nil {
			err = fmt.Errorf("could not process child: %w", err)
			result = multierror.Append(result, err)
		}
	}

	// drop all of the children that should have been processed now
	e.buffer.DropForParent(parentID)

	return result.ErrorOrNil()
}
