package blocks

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit              *engine.Unit
	log               zerolog.Logger
	me                module.Local
	state             protocol.State
	conduit           network.Conduit
	collectionConduit network.Conduit
	blocks            storage.Blocks
	collections       storage.Collections
	execution         network.Engine
	mempool           *Mempool
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	executionEngine network.Engine,
	mempool *Mempool,
) (*Engine, error) {

	eng := Engine{
		unit:        engine.NewUnit(),
		log:         logger,
		me:          me,
		state:       state,
		blocks:      blocks,
		collections: collections,
		execution:   executionEngine,
		mempool:     mempool,
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

func (e *Engine) findCollectionNodes() ([]flow.Identifier, error) {
	identities, err := e.state.Final().Identities(identity.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities: %w", err)
	}
	if len(identities) < 1 {
		return nil, fmt.Errorf("no Collection identity found")
	}
	identifiers := make([]flow.Identifier, len(identities))
	for i, id := range identities {
		identifiers[i] = id.NodeID
	}
	return identifiers, nil
}

func (e *Engine) checkForCompleteness(block *execution.CompleteBlock) bool {

	for _, collection := range block.Block.Guarantees {

		completeCollection, ok := block.CompleteCollections[collection.ID()]
		if ok && completeCollection.Transactions != nil {
			continue
		}
		return false
	}

	return true
}

func (e *Engine) removeCollections(block *execution.CompleteBlock, backdata *Backdata) {

	for _, collection := range block.Block.Guarantees {
		backdata.Rem(collection.ID())
	}
}

func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_number", block.Number).
		Msg("received block")

	err := e.blocks.Store(block)
	if err != nil {
		return fmt.Errorf("could not save block: %w", err)
	}

	blockID := block.ID()

	collectionIdentifiers, err := e.findCollectionNodes()
	if err != nil {
		return err
	}

	emptyCompleteBlock := &execution.CompleteBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*execution.CompleteCollection),
	}

	err = e.mempool.Run(func(backdata *Backdata) error {
		for _, guarantee := range block.Guarantees {
			completeBlock, err := backdata.Get(guarantee.ID())
			if err == mempool.ErrEntityNotFound {
				emptyCompleteBlock.CompleteCollections[guarantee.ID()] = &execution.CompleteCollection{
					Guarantee:    guarantee,
					Transactions: nil,
				}
				err := backdata.Add(&blockByCollection{
					CollectionID: guarantee.ID(),
					Block:        emptyCompleteBlock,
				})
				if err != nil {
					return fmt.Errorf("cannot save collection-block mapping: %w", err)
				}

				err = e.collectionConduit.Submit(messages.CollectionRequest{ID: guarantee.ID()}, collectionIdentifiers...)
				if err != nil {
					e.log.Err(err).Msg("cannot submit collection requests")
				}
				continue
			}
			if err != nil {
				return fmt.Errorf("cannot get an item from mempool: %w", err)
			}
			if completeBlock.ID() != blockID {
				// Should not happen in MVP
				return fmt.Errorf("received block with same collection alredy pointing to different block ")
			}
		}
		return nil
	})

	return err
}

func (e *Engine) handleCollectionResponse(response *messages.CollectionResponse) error {

	collection := response.Collection

	e.log.Debug().
		Hex("collection_id", logging.Entity(collection)).
		Msg("received collection")

	collID := collection.ID()

	return e.mempool.Run(func(backdata *Backdata) error {
		completeBlock, err := backdata.Get(collID)
		if err == nil {
			if completeCollection, ok := completeBlock.Block.CompleteCollections[collID]; ok {
				if completeCollection.Transactions == nil {
					completeCollection.Transactions = collection.Transactions
					if e.checkForCompleteness(completeBlock.Block) {
						e.removeCollections(completeBlock.Block, backdata)
						e.execution.SubmitLocal(completeBlock.Block)
					}
				}
			} else {
				return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
			}
		}
		return nil
	})
}
