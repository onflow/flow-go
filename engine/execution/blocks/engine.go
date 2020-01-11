package blocks

import (
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit              *engine.Unit
	log               zerolog.Logger
	conduit           network.Conduit
	collectionConduit network.Conduit
	me                module.Local
	blocks            storage.Blocks
	collections       storage.Collections
	state             protocol.State
	execution         network.Engine

	pendingBlocks      map[flow.Identifier]*execution.CompleteBlock
	pendingCollections map[flow.Identifier]flow.Identifier
}

func New(logger zerolog.Logger, net module.Network, me module.Local, blocks storage.Blocks, collections storage.Collections, state protocol.State, executionEngine network.Engine) (*Engine, error) {
	eng := Engine{
		unit:               engine.NewUnit(),
		log:                logger,
		me:                 me,
		blocks:             blocks,
		collections:        collections,
		state:              state,
		execution:          executionEngine,
		pendingBlocks:      make(map[flow.Identifier]*execution.CompleteBlock),
		pendingCollections: make(map[flow.Identifier]flow.Identifier),
	}

	con, err := net.Register(engine.BlockProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	collConduit, err := net.Register(engine.CollectionProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register collection provider engine")
	}

	eng.conduit = con
	eng.collectionConduit = collConduit

	return &eng, nil
}

func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			e.log.Error().Err(err).Msg("could not process submitted event")
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {

	return e.unit.Do(func() error {
		var err error
		switch v := event.(type) {
		case *flow.Block:
			err = e.handleBlock(v)
		case *messages.CollectionResponse:
			err = e.handleCollectionResponse(v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
}

func (e *Engine) findCollectionNode() (flow.Identifier, error) {
	identities, err := e.state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return flow.Identifier{}, fmt.Errorf("could not retrieve identities: %w", err)
	}
	if len(identities) < 1 {
		return flow.Identifier{}, fmt.Errorf("no Collection identity found")
	}
	return identities[rand.Intn(len(identities))].NodeID, nil
}

func (e *Engine) checkForCompleteness(block *execution.CompleteBlock) bool {

	for _, collection := range block.Block.Guarantees {

		completeCollection, ok := block.CompleteCollections[collection.ID()]
		if ok && completeCollection.Transactions != nil {
			continue
		}
		return false
	}

	for _, collection := range block.Block.Guarantees {
		delete(e.pendingCollections, collection.ID())
	}
	delete(e.pendingBlocks, block.Block.ID())

	return true
}

func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Debug().
		Hex("block_id", logging.ID(block)).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Store(block)
	if err != nil {
		return fmt.Errorf("could not save block: %w", err)
	}

	blockID := block.ID()

	if _, ok := e.pendingBlocks[blockID]; !ok {

		randomCollectionIdentifier, err := e.findCollectionNode()
		if err != nil {
			return err
		}

		completeBlock := &execution.CompleteBlock{
			Block:               *block,
			CompleteCollections: make(map[flow.Identifier]*execution.CompleteCollection),
		}

		for _, collection := range block.Guarantees {
			id := collection.ID()
			e.pendingCollections[id] = blockID
			completeBlock.CompleteCollections[id] = &execution.CompleteCollection{
				Collection:   collection,
				Transactions: nil,
			}

			err := e.collectionConduit.Submit(messages.CollectionRequest{ID: collection.ID()}, randomCollectionIdentifier)
			if err != nil {
				e.log.Err(err).Msg("cannot submit collection requests")
			}
		}

		e.pendingBlocks[blockID] = completeBlock
	}

	return nil
}

func (e *Engine) handleCollectionResponse(response *messages.CollectionResponse) error {

	collection := response.Collection

	e.log.Debug().
		Hex("collection_id", logging.ID(collection)).
		Msg("received collection")

	collID := collection.ID()

	if blockHash, ok := e.pendingCollections[collID]; ok {
		if block, ok := e.pendingBlocks[blockHash]; ok {
			if completeCollection, ok := block.CompleteCollections[collID]; ok {
				if completeCollection.Transactions == nil {
					completeCollection.Transactions = collection.Transactions
					if e.checkForCompleteness(block) {
						e.execution.SubmitLocal(block)
					}
				}
			}
		} else {
			return fmt.Errorf("cannot handle collection: internal inconsistency - pending collection pointing to non-existing block")
		}
	}

	return nil
}
