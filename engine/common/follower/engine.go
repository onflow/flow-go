package follower

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
	"github.com/dapperlabs/flow-go/storage"
)

type Engine struct {
	unit  *engine.Unit
	log   zerolog.Logger
	me    module.Local
	state protocol.State
	con   network.Conduit

	headers  storage.Headers
	payloads storage.Payloads

	cache module.PendingBlockBuffer

	follower module.HotStuffFollower
}

type cacheItem struct {
	OriginID flow.Identifier
	Proposal *messages.ClusterBlockProposal
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

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For consensus engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the consensus engine, we wait for hotstuff to finish.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
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

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.Block:
		return e.onBlock(originID, ev)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onBlock(originID flow.Identifier, block *flow.Block) error {

	// retrieve the parent for the block, cache if not found
	parent, err := e.headers.ByBlockID(block.ParentID)
	if errors.Is(err, storage.ErrNotFound) {
		return e.processPendingBlock(originID, block)
	}
	if err != nil {
		return fmt.Errorf("could not retrieve block parent: %w", err)
	}

	// store the block payload
	err = e.payloads.Store(&block.Header, &block.Payload)
	if err != nil {
		return fmt.Errorf("could not store block payload: %w", err)
	}

	// insert the block header
	err = e.headers.Store(&block.Header)
	if err != nil {
		return fmt.Errorf("could not store block header: %w", err)
	}

	// ensure the block is a valid extension of protocol state
	err = e.state.Mutate().Extend(block.ID())
	if err != nil {
		return fmt.Errorf("could not extend protocol state: %w", err)
	}

	// submit the model to hotstuff finalization logic
	e.follower.SubmitProposal(&block.Header, parent.View)

	// check for any descendants of the block that are now processable
	children, ok := e.cache.ByParentID(parent.ID())
	if !ok {
		return nil
	}

	// then try to process children only this once
	var result *multierror.Error
	for _, child := range children {
		block := flow.Block{
			Header:  *child.Header,
			Payload: *child.Payload,
		}
		err := e.onBlock(child.OriginID, &block)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	// remove children from the cache
	e.cache.DropForParent(parent.ID())

	return result.ErrorOrNil()
}

func (e *Engine) processPendingBlock(originID flow.Identifier, block *flow.Block) error {

	pendingBlock := &flow.PendingBlock{
		OriginID: originID,
		Header:   &block.Header,
		Payload:  &block.Payload,
	}

	// add the block to the buffer
	exists := e.cache.Add(pendingBlock)
	if exists {
		return nil
	}

	// if the block was not already in the buffer, request its parent
	req := &messages.BlockRequest{
		BlockID: block.ParentID,
		Nonce:   rand.Uint64(),
	}
	err := e.con.Submit(req, originID)
	if err != nil {
		return fmt.Errorf("could not send block request: %w", err)
	}

	return nil
}
