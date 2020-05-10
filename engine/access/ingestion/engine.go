// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"errors"
	"fmt"
	"math/rand"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	unit    *engine.Unit   // used to manage concurrency & shutdown
	log     zerolog.Logger // used to log relevant actions with context
	metrics module.Metrics // used to collect metrics
	state   protocol.State // used to access the  protocol state
	me      module.Local   // used to access local node information

	// Conduits
	collectionConduit network.Conduit

	// storage
	// FIX: remove direct DB access by substituting indexer module
	blocks       storage.Blocks
	headers      storage.Headers
	collections  storage.Collections
	transactions storage.Transactions
}

// New creates a new access ingestion engine
func New(log zerolog.Logger,
	net module.Network,
	state protocol.State,
	metrics module.Metrics,
	me module.Local,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	eng := &Engine{
		unit:         engine.NewUnit(),
		log:          log.With().Str("engine", "ingestion").Logger(),
		metrics:      metrics,
		state:        state,
		me:           me,
		blocks:       blocks,
		headers:      headers,
		collections:  collections,
		transactions: transactions,
	}

	collConduit, err := net.Register(engine.CollectionProvider, eng)
	if err != nil {
		return nil, fmt.Errorf("could not register collection provider engine: %w", err)
	}

	eng.collectionConduit = collConduit

	return eng, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
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
		err := e.process(originID, event)
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

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *messages.BlockProposal:
		return e.onBlockProposal(originID, entity)
	case *messages.CollectionResponse:
		return e.handleCollectionResponse(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated
func (e *Engine) OnFinalizedBlock(hb *model.Block) {
	e.unit.Launch(func() {
		id := hb.BlockID
		block, err := e.blocks.ByID(id)
		if err != nil {
			e.log.Error().Err(err).Hex("block_id", id[:]).Msg("failed to lookup block")
			return
		}
		proposal := &messages.BlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}
		err = e.ProcessLocal(proposal)
		if err != nil {
			e.log.Error().Err(err).Hex("block_id", id[:]).Msg("failed to process block")
			return
		}
		e.metrics.FollowerFinalizedBlockHeight(hb.View)
	})
}

// onBlock handles an incoming block.
// TODO this will be an event triggered by the follower node when a new finalized or sealed block is received
func (e *Engine) onBlockProposal(_ flow.Identifier, proposal *messages.BlockProposal) error {

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// TODO: substitute an indexer module as layer between engine and storage

	// index the block storage with each of the collection guarantee
	err := e.blocks.IndexBlockForCollections(proposal.Header.ID(), flow.GetIDs(proposal.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// request each of the collections from the collection node
	return e.requestCollections(proposal.Payload.Guarantees...)
}

// handleCollectionResponse handles the response of the a collection request made earlier when a block was received
func (e *Engine) handleCollectionResponse(originID flow.Identifier, response *messages.CollectionResponse) error {
	collection := response.Collection
	light := collection.Light()

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err := e.collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			return nil
		}
		return err
	}

	// now store each of the transaction body
	for _, tx := range collection.Transactions {
		err := e.transactions.Store(tx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) requestCollections(guarantees ...*flow.CollectionGuarantee) error {
	ids, err := e.findCollectionNodes()
	if err != nil {
		return err
	}

	// Request all the collections for this block
	for _, g := range guarantees {
		err := e.collectionConduit.Submit(&messages.CollectionRequest{ID: g.ID(), Nonce: rand.Uint64()}, ids...)
		if err != nil {
			return err
		}
	}

	return nil

}

func (e *Engine) findCollectionNodes() ([]flow.Identifier, error) {
	identities, err := e.state.Final().Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}
	if len(identities) < 1 {
		return nil, fmt.Errorf("no Collection identity found")
	}
	identifiers := flow.GetIDs(identities)
	return identifiers, nil
}

// OnBlockIncorporated is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnBlockIncorporated(*model.Block) {
}

// OnDoubleProposeDetected is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {
}
