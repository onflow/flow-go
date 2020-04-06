package follower

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

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
	state    protocol.State
	headers  storage.Headers
	payloads storage.Payloads
	cache    module.PendingBlockBuffer
	follower module.HotStuffFollower
	con      network.Conduit
	sync     module.Synchronization
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	headers storage.Headers,
	payloads storage.Payloads,
	cache module.PendingBlockBuffer,
	follower module.HotStuffFollower,
) (*Engine, error) {

	e := &Engine{
		unit:     engine.NewUnit(),
		log:      log.With().Str("engine", "follower").Logger(),
		me:       me,
		state:    state,
		headers:  headers,
		payloads: payloads,
		cache:    cache,
		follower: follower,
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
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
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
		Header:  &synced.Block.Header,
		Payload: &synced.Block.Payload,
	}
	return e.onBlockProposal(originID, proposal)
}

func (e *Engine) onBlockProposal(originID flow.Identifier, proposal *messages.BlockProposal) error {

	// get the latest finalized block
	final, err := e.state.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get final block: %w", err)
	}

	// check if the block is connected to the current finalized state
	finalID := final.ID()
	ancestorID := proposal.Header.ParentID
	for ancestorID != finalID {
		ancestor, ok := e.cache.ByID(ancestorID)
		if !ok {
			return e.handleMissingAncestor(originID, proposal, ancestorID)
		}
		if ancestor.Header.Height <= final.Height {
			// TODO: store this in future versions for slashing challenges
			return fmt.Errorf("rejecting orphaned block (%x)", proposal.Header.ID())
		}
		ancestorID = ancestor.Header.ParentID
	}

	// store the proposal block payload
	err = e.payloads.Store(proposal.Header, proposal.Payload)
	if err != nil {
		return fmt.Errorf("could not store block payload: %w", err)
	}

	// store the proposal block header
	err = e.headers.Store(proposal.Header)
	if err != nil {
		return fmt.Errorf("could not store header: %w", err)
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

	// submit the model to hotstuff finalization logic
	e.follower.SubmitProposal(proposal.Header, parent.View)

	// check for any descendants of the block to process
	return e.handleConnectedChildren(blockID)
}

func (e *Engine) handleMissingAncestor(originID flow.Identifier, proposal *messages.BlockProposal, ancestorID flow.Identifier) error {

	pendingBlock := &flow.PendingBlock{
		OriginID: originID,
		Header:   proposal.Header,
		Payload:  proposal.Payload,
	}

	// cache the block, exit early if it already exists in the cache
	added := e.cache.Add(pendingBlock)
	if !added {
		return nil
	}

	// if the block was not already in the buffer, request its parent
	e.sync.RequestBlock(ancestorID)

	return nil
}

// handleConnectedChildren checks if the given block has connected some children
// that were missing a link to the finalized state, in order to process them as
// well.
func (e *Engine) handleConnectedChildren(blockID flow.Identifier) error {

	// check if there are any children for this parent in the cache
	children, ok := e.cache.ByParentID(blockID)
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
		err := e.onBlockProposal(child.OriginID, proposal)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove the children from cache
	e.cache.DropForParent(blockID)

	return result.ErrorOrNil()
}
