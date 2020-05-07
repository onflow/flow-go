package follower

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	me       module.Local
	cleaner  storage.Cleaner
	headers  storage.Headers
	payloads storage.Payloads
	state    protocol.State
	pending  module.PendingBlockBuffer
	follower module.HotStuffFollower
	con      network.Conduit
	sync     module.Synchronization
	metrics  module.Metrics
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	cleaner storage.Cleaner,
	headers storage.Headers,
	payloads storage.Payloads,
	state protocol.State,
	pending module.PendingBlockBuffer,
	follower module.HotStuffFollower,
	metrics module.Metrics,
) (*Engine, error) {

	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "follower").Logger(),
		me:       me,
		cleaner:  cleaner,
		headers:  headers,
		payloads: payloads,
		state:    state,
		pending:  pending,
		follower: follower,
		metrics:  metrics,
	}

	con, err := net.Register(engine.BlockProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine to network: %w", err)
	}
	e.con = con

	return e, nil
}

// WithSynchronization injects the given synchronization protocol into the
// hotstuff follower, providing it with blocks proactively, while also allowing
// it to explicitly request blocks by ID.
func (e *Engine) WithSynchronization(sync module.Synchronization) *Engine {
	e.sync = sync
	return e
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	if e.sync == nil {
		panic("follower engine requires injected synchronization module")
	}
	return e.unit.Ready(func() {
		<-e.sync.Ready()
		<-e.follower.Ready()
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.follower.Done()
		<-e.sync.Done()
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

func (e *Engine) process(originID flow.Identifier, input interface{}) error {
	switch v := input.(type) {
	case *events.SyncedBlock:
		return e.onSyncedBlock(originID, v)
	case *messages.BlockProposal:
		return e.onBlockProposal(originID, v)
	default:
		return fmt.Errorf("invalid event type (%T)", input)
	}
}

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
	return e.onBlockProposal(originID, proposal)
}

// onBlockProposal handles incoming block proposals.
func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {
	e.metrics.NewestKnownQC(proposal.Header.View)

	blockID := proposal.Header.ID()
	log := e.log.With().
		Uint64("block_view", proposal.Header.View).
		Hex("block_id", blockID[:]).
		Uint64("block_height", proposal.Header.Height).
		Hex("parent_id", proposal.Header.ParentID[:]).
		Hex("signer", proposal.Header.ProposerID[:]).
		Logger()

	log.Info().Msg("block proposal received")

	// get the latest finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get final block: %w", err)
	}

	// reject blocks that are outdated
	if proposal.Header.Height <= final.Height {
		log.Debug().Msg("skipping proposal with outdated view")
		return nil
	}

	// skip a proposal that is already cached
	_, cached := e.pending.ByID(blockID)
	if cached {
		log.Debug().Msg("skipping already cached proposal")
		return nil
	}

	// skip a proposal that is already persisted as pending
	_, err = e.headers.ByBlockID(blockID)
	if err == nil {
		log.Debug().Msg("skipping already pending proposal")
		return nil
	}

	// there are three possibilities from here on:
	// 1) the proposal connects to a block in the pending cache
	// => cache the proposal and (re-)request the missing ancestor
	// 2) the proposal's parent does not exist in the protocol state
	// => cache the proposal and request the missing parent
	// 3) the proposal can be processed for validity
	// => process the block for validity and forward to hotstuff

	// if we connect to the cache, it means we are definitely missing a link and
	// we should add to the pending and request this missing link
	ancestor, found := e.pending.ByID(proposal.Header.ParentID)
	if found {

		// add the block to the cache (double check guards against race condition)
		added := e.pending.Add(originID, proposal)
		if !added {
			log.Warn().Msg("race condition on adding proposal to cache")
			return nil
		}

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

	// if the parent is not part of persistent state, we also should request the
	// missing link
	_, err = e.headers.ByBlockID(proposal.Header.ParentID)
	if err == storage.ErrNotFound {

		log.Debug().Msg("requesting missing parent for proposal")

		// add the block to the cache (double check guards against race condition)
		added := e.pending.Add(originID, proposal)
		if !added {
			log.Warn().Msg("race condition on adding proposal to cache")
			return nil
		}

		e.sync.RequestBlock(proposal.Header.ParentID)

		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get parent: %w", err)
	}

	// process the block proposal now
	err = e.processBlockProposal(proposal)
	if err != nil {
		return fmt.Errorf("could not process block proposal: %w", err)
	}

	log.Info().Msg("block proposal processed")

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

	// store the proposal block payload
	err := e.payloads.Store(proposal.Header, proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store proposal payload: %w", err)
	}

	// store the proposal block header
	err = e.headers.Store(proposal.Header)
	if err != nil {
		return fmt.Errorf("could not store proposal header: %w", err)
	}

	// see if the block is a valid extension of the protocol state
	blockID := proposal.Header.ID()
	err = e.state.Mutate().Extend(blockID)
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	// retrieve the parent
	parent, err := e.headers.ByBlockID(proposal.Header.ParentID)
	if err != nil {
		return fmt.Errorf("could not retrieve proposal parent: %w", err)
	}

	log.Info().
		Uint64("block_view", proposal.Header.View).
		Hex("block_id", blockID[:]).
		Uint64("block_height", proposal.Header.Height).
		Hex("parent_id", proposal.Header.ParentID[:]).
		Hex("signer", proposal.Header.ProposerID[:]).
		Msg("forwarding block proposal to hotstuff follower")

	// submit the model to hotstuffFollower for processing
	e.follower.SubmitProposal(proposal.Header, parent.View)

	// check for any descendants of the block to process
	return e.processPendingChildren(proposal.Header)
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
