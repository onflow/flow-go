package ingestion

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
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
	payloads          storage.Payloads
	collections       storage.Collections
	execution         computation.ComputationEngine
	mempool           *Mempool
	execState         state.ExecutionState
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	payloads storage.Payloads,
	collections storage.Collections,
	executionEngine computation.ComputationEngine,
	execState state.ExecutionState,
) (*Engine, error) {
	log := logger.With().Str("engine", "blocks").Logger()

	mempool, err := newMempool()
	if err != nil {
		return nil, errors.Wrap(err, "could not create mempool")
	}

	eng := Engine{
		unit:        engine.NewUnit(),
		log:         log,
		me:          me,
		state:       state,
		blocks:      blocks,
		payloads:    payloads,
		collections: collections,
		execution:   executionEngine,
		mempool:     mempool,
		execState:   execState,
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
	identities, err := e.state.Final().Identities(filter.HasRole(flow.RoleCollection))
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

func (e *Engine) isComplete(block *execution.CompleteBlock) bool {

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
		Uint64("block_view", block.View).
		Msg("received block")

	err := e.payloads.Store(&block.Payload)
	if err != nil {
		return fmt.Errorf("could not save block payload: %w", err)
	}

	err = e.blocks.Store(block)
	if err != nil {
		return fmt.Errorf("could not save block: %w", err)
	}

	blockID := block.ID()

	// TODO: for MVP assume we're only receiving finalized blocks
	err = e.state.Mutate().Finalize(blockID)
	if err != nil {
		return fmt.Errorf("could not finalize block: %w", err)
	}

	collectionIdentifiers, err := e.findCollectionNodes()
	if err != nil {
		return err
	}

	maybeCompleteBlock := &execution.CompleteBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*execution.CompleteCollection),
	}

	err = e.mempool.Run(func(backdata *Backdata) error {
		// In case we have all the collections, or the block is empty
		if e.isComplete(maybeCompleteBlock) {
			e.removeCollections(maybeCompleteBlock, backdata)
			e.handleCompleteBlock(maybeCompleteBlock)
			return nil
		}

		for _, guarantee := range block.Guarantees {
			completeBlock, err := backdata.ByID(guarantee.ID())
			if err == mempool.ErrEntityNotFound {
				maybeCompleteBlock.CompleteCollections[guarantee.ID()] = &execution.CompleteCollection{
					Guarantee:    guarantee,
					Transactions: nil,
				}
				err := backdata.Add(&blockByCollection{
					CollectionID: guarantee.ID(),
					Block:        maybeCompleteBlock,
				})
				if err != nil {
					return fmt.Errorf("cannot save collection-block mapping: %w", err)
				}

				e.log.Debug().
					Hex("block_id", logging.Entity(block)).
					Hex("collection_id", logging.ID(guarantee.ID())).
					Msg("requesting collection")

				err = e.collectionConduit.Submit(&messages.CollectionRequest{ID: guarantee.ID()}, collectionIdentifiers...)
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

func (e *Engine) handleCompleteBlock(completeBlock *execution.CompleteBlock) {

	//get initial start state from parent block
	startState, err := e.execState.StateCommitmentByBlockID(completeBlock.Block.ParentID)

	if err != nil {
		e.log.Err(err).
			Hex("parent_block_id", logging.ID(completeBlock.Block.ParentID)).
			Msg("error while fetching state commitment")
		return
	}

	view := e.execState.NewView(startState)

	e.execution.SubmitLocal(&execution.ComputationOrder{
		Block: completeBlock,
		View:  view,
	})
}

func (e *Engine) ExecuteScript(script []byte) ([]byte, error) {

	seal, err := e.state.Final().Seal()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest seal: %w", err)
	}

	stateCommit, err := e.execState.StateCommitmentByBlockID(seal.BlockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", seal.BlockID, err)
	}
	block, err := e.state.AtBlockID(seal.BlockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get sealed block (%s): %w", seal.BlockID, err)
	}

	blockView := e.execState.NewView(stateCommit)

	return e.execution.ExecuteScript(script, block, blockView)
}

func (e *Engine) handleCollectionResponse(response *messages.CollectionResponse) error {

	collection := response.Collection

	e.log.Debug().
		Hex("collection_id", logging.Entity(collection)).
		Msg("received collection")

	collID := collection.ID()

	return e.mempool.Run(func(backdata *Backdata) error {
		completeBlock, err := backdata.ByID(collID)
		if err != nil {
			return err
		}
		completeCollection, ok := completeBlock.Block.CompleteCollections[collID]
		if !ok {
			return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
		}
		// already received transactions for this collection
		// TODO - check if data stored is the same
		if completeCollection.Transactions != nil {
			return nil
		}

		completeCollection.Transactions = collection.Transactions
		if !e.isComplete(completeBlock.Block) {
			return nil
		}

		e.removeCollections(completeBlock.Block, backdata)
		e.handleCompleteBlock(completeBlock.Block)
		return nil
	})
}
