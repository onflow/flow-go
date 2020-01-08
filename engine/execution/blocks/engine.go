package blocks

import (
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/collection/provider"
	"github.com/dapperlabs/flow-go/engine/execution/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
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
	execution         execution.ExecutionEngine

	//TODO change for proper fingerprint/hash types once they become usable as maps keys
	pendingBlocks      map[string]execution.CompleteBlock
	pendingCollections map[string]string
}

func New(logger zerolog.Logger, net module.Network, me module.Local, blocks storage.Blocks, collections storage.Collections, state protocol.State, executionEngine execution.ExecutionEngine) (*Engine, error) {
	eng := Engine{
		unit:               engine.NewUnit(),
		log:                logger,
		me:                 me,
		blocks:             blocks,
		collections:        collections,
		state:              state,
		execution:          executionEngine,
		pendingBlocks:      map[string]execution.CompleteBlock{},
		pendingCollections: map[string]string{},
	}

	con, err := net.Register(engine.ExecutionBlockIngestion, &eng)
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
		case flow.Block:
			err = e.handleBlock(v)
		case provider.CollectionResponse:
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
	identities, err := e.state.Final().Identities(func(identity flow.Identity) bool {
		return identity.Role == flow.RoleCollection
	})
	if err != nil {
		return flow.Identifier{}, err
	}
	if len(identities) < 1 {
		return flow.Identifier{}, fmt.Errorf("no Collection identity found")
	}
	return identities[rand.Intn(len(identities))].NodeID, nil
}

func (e *Engine) checkForCompleteness(block execution.CompleteBlock) error {

	for _, collection := range block.Block.CollectionGuarantees {
		//TODO change to proper hash once it can be used as maps key
		hash := string(collection.Hash)

		if _, ok := block.CompleteCollections[hash]; !ok {
			return nil
		}
	}

	for _, collection := range block.Block.CollectionGuarantees {
		hash := string(collection.Hash)
		delete(e.pendingCollections, hash)
	}
	blockHash := string(block.Block.Hash())
	delete(e.pendingBlocks, blockHash)

	//TODO results storage?
	return e.execution.ExecuteBlock(block)
}

func (e *Engine) handleBlock(block flow.Block) error {
	e.log.Debug().
		Hex("block_hash", block.Hash()).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Save(&block)
	if err != nil {
		return fmt.Errorf("could not save block: %w", err)
	}

	blockHash := string(block.Hash())

	if _, ok := e.pendingBlocks[blockHash]; !ok {

		randomCollectionIdentifier, err := e.findCollectionNode()
		if err != nil {
			return err
		}

		completeBlock := execution.CompleteBlock{
			Block:               block,
			CompleteCollections: map[string]execution.CompleteCollection{},
		}

		for _, collection := range block.CollectionGuarantees {
			hash := string(collection.Fingerprint())
			e.pendingCollections[hash] = blockHash
			completeBlock.CompleteCollections[hash] = execution.CompleteCollection{
				Collection:   *collection,
				Transactions: nil,
			}

			err := e.collectionConduit.Submit(provider.CollectionRequest{Fingerprint: collection.Fingerprint()}, randomCollectionIdentifier)
			if err != nil {
				e.log.Err(err).Msg("cannot submit collection requests")
			}
		}

		e.pendingBlocks[blockHash] = completeBlock
	}

	return nil
}

func (e *Engine) handleCollectionResponse(collectionResponse provider.CollectionResponse) error {
	e.log.Debug().
		Hex("collection_hash", collectionResponse.Fingerprint).
		Msg("received collection")

	//err := e.collections.Save(&collection)
	//if err != nil {
	//	return fmt.Errorf("could not save collection: %w", err)
	//}

	hash := string(collectionResponse.Fingerprint)

	if blockHash, ok := e.pendingCollections[hash]; ok {
		if block, ok := e.pendingBlocks[blockHash]; ok {
			if completeCollection, ok := block.CompleteCollections[hash]; ok {
				if completeCollection.Transactions == nil {
					completeCollection.Transactions = collectionResponse.Transactions
					return e.checkForCompleteness(block)
				}
			}
		} else {
			return fmt.Errorf("cannot handle collection: internal inconsistency - pending collection pointing to non-existing block")
		}
	}

	return nil
}
