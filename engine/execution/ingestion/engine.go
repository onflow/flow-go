package ingestion

import (
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
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
	computationEngine computation.ComputationEngine
	providerEngine    provider.ProviderEngine
	mempool           *Mempool
	execState         state.ExecutionState
	wg                sync.WaitGroup
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
	providerEngine provider.ProviderEngine,
	execState state.ExecutionState,
) (*Engine, error) {
	log := logger.With().Str("engine", "blocks").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:              engine.NewUnit(),
		log:               log,
		me:                me,
		state:             state,
		blocks:            blocks,
		payloads:          payloads,
		collections:       collections,
		computationEngine: executionEngine,
		providerEngine:    providerEngine,
		mempool:           mempool,
		execState:         execState,
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

// Engine boilerplate
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
	return e.unit.Done(func() {
		e.wg.Wait() //wait for block execution
	})
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

// Main handling

func (e *Engine) handleBlock(block *flow.Block) error {

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_view", block.View).
		Msg("received block")

	err := e.blocks.Store(block)
	if err != nil {
		return fmt.Errorf("could not store block: %w", err)
	}

	// TODO: for MVP assume we're only receiving finalized blocks
	// but in essence, Execution Node doesn't care about finalization of blocks.
	// However, for executing scripts we need latest finalized state and
	// we need protocol state to be able to find other nodes
	// Once the Consensus Follower is ready to be implemented, this should be replaced
	blockID := block.Header.ID()
	err = e.state.Mutate().Finalize(blockID)
	if err != nil {
		return fmt.Errorf("could not finalize block: %w", err)
	}

	completeBlock := &execution.CompleteBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*execution.CompleteCollection),
	}

	err = e.mempool.Run(func(blockByCollection *BlockByCollectionBackdata, executionQueue *QueuesBackdata, orphanQueue *QueuesBackdata) error {

		err := e.sendCollectionsRequest(completeBlock, blockByCollection)
		if err != nil {
			return fmt.Errorf("cannot send collction requests: %w", err)
		}

		// if block fits into execution queue, that's it
		if queue, added := tryEnqueue(completeBlock, executionQueue); added {
			e.tryRequeueOrphans(completeBlock, queue, orphanQueue)
			return nil
		}

		// if block fits into orphan queues
		if queue, added := tryEnqueue(completeBlock, orphanQueue); added {
			e.tryRequeueOrphans(completeBlock, queue, orphanQueue)
			return nil
		}

		stateCommitment, err := e.execState.StateCommitmentByBlockID(block.ParentID)
		// if state commitment doesn't exist and there are no known blocks which will produce
		// it soon (execution queue) that we save it as orphaned
		if err == storage.ErrNotFound {
			_, err := enqueue(completeBlock, orphanQueue)
			if err != nil {
				return fmt.Errorf("cannot add orphaned block: %w", err)
			}
			return nil
		}
		// any other error while accessing storage - panic
		if err != nil {
			panic(fmt.Sprintf("unexpected error while accessing storage, shutting down: %v", err))
		}

		completeBlock.StartState = stateCommitment
		newQueue, err := enqueue(completeBlock, executionQueue)
		if err != nil {
			return fmt.Errorf("cannot enqueue block for execution: %w", err)
		}
		// If the block was empty
		if completeBlock.IsComplete() {
			e.tryRequeueOrphans(completeBlock, newQueue, orphanQueue)
			e.wg.Add(1)
			go e.executeBlock(completeBlock)
		}

		return nil
	})

	return err
}

// tryRequeueOrphans tries to put orphaned queue into execution queue after a new block has been added
func (e *Engine) tryRequeueOrphans(completeBlock *execution.CompleteBlock, targetQueue *Queue, potentialQueues *QueuesBackdata) {
	for _, queue := range potentialQueues.All() {
		// only need to check for heads, as all children has parent already
		// there might be many queues sharing a parent
		if queue.Head.CompleteBlock.Block.ParentID == completeBlock.Block.ID() {
			err := targetQueue.Attach(queue)
			// shouldn't happen
			if err != nil {
				panic(fmt.Sprintf("internal error while joining queues"))
			}
			potentialQueues.Rem(queue.ID())
		}
	}
}

func (e *Engine) executeBlock(completeBlock *execution.CompleteBlock) {
	defer e.wg.Done()

	view := e.execState.NewView(completeBlock.StartState)
	e.log.Info().
		Hex("block_id", logging.Entity(completeBlock.Block)).
		Msg("executing block")

	computationResult, err := e.computationEngine.ComputeBlock(completeBlock, view)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(completeBlock.Block)).
			Msg("error while computing block")
		return
	}

	finalState, err := e.handleComputationResult(computationResult, completeBlock.StartState)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(completeBlock.Block)).
			Msg("error while handing computation results")
		return
	}

	err = e.mempool.ExecutionQueue.Run(func(executionQueues *QueuesBackdata) error {
		executionQueue, err := executionQueues.ByID(completeBlock.Block.ID())
		if err != nil {
			return fmt.Errorf("fatal error - executed block not present in execution queue: %w", err)
		}
		_, newQueues := executionQueue.Dismount()
		for _, queue := range newQueues {
			queue.Head.CompleteBlock.StartState = finalState
			err := executionQueues.Add(queue)
			if err != nil {
				return fmt.Errorf("fatal error cannot add children block to execution queue: %w", err)
			}
			if queue.Head.CompleteBlock.IsComplete() {
				e.wg.Add(1)
				go e.executeBlock(queue.Head.CompleteBlock)
			}
		}
		executionQueues.Rem(completeBlock.Block.ID())
		return nil
	})

	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(completeBlock.Block)).
			Msg("error while requeueing blocks after execution")
	}
}

func (e *Engine) handleCollectionResponse(response *messages.CollectionResponse) error {

	collection := response.Collection

	e.log.Debug().
		Hex("collection_id", logging.Entity(collection)).
		Msg("received collection")

	collID := collection.ID()

	return e.mempool.BlockByCollection.Run(func(backdata *BlockByCollectionBackdata) error {
		blockByCollectionId, err := backdata.ByID(collID)
		if err != nil {
			return err
		}
		completeBlock := blockByCollectionId.CompleteBlock

		completeCollection, ok := completeBlock.CompleteCollections[collID]
		if !ok {
			return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
		}
		// already received transactions for this collection
		// TODO - check if data stored is the same
		if completeCollection.Transactions != nil {
			return nil
		}

		completeCollection.Transactions = collection.Transactions
		if completeBlock.HasAllTransactions() {
			e.clearCollectionsCache(completeBlock, backdata)
		}

		if completeBlock.IsComplete() {
			e.wg.Add(1)
			go e.executeBlock(completeBlock)
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

func (e *Engine) clearCollectionsCache(block *execution.CompleteBlock, backdata *BlockByCollectionBackdata) {
	for _, collection := range block.Block.Guarantees {
		backdata.Rem(collection.ID())
	}
}

// tryEnqueue checks if a block fits somewhere into the already existing queues, and puts it there is so
func tryEnqueue(completeBlock *execution.CompleteBlock, queues *QueuesBackdata) (*Queue, bool) {
	for _, queue := range queues.All() {
		if queue.TryAdd(completeBlock) {
			return queue, true
		}
	}
	return nil, false
}

func newQueue(completeBlock *execution.CompleteBlock, queue *QueuesBackdata) (*Queue, error) {
	q := NewQueue(completeBlock)
	return q, queue.Add(q)
}

// enqueue inserts block into matching queue or creates a new one
func enqueue(completeBlock *execution.CompleteBlock, queues *QueuesBackdata) (*Queue, error) {
	for _, queue := range queues.All() {
		if queue.TryAdd(completeBlock) {
			return queue, nil
		}
	}
	return newQueue(completeBlock, queues)
}

func (e *Engine) sendCollectionsRequest(completeBlock *execution.CompleteBlock, backdata *BlockByCollectionBackdata) error {

	collectionIdentifiers, err := e.findCollectionNodes()
	if err != nil {
		return err
	}

	for _, guarantee := range completeBlock.Block.Guarantees {
		maybeBlockByCollection, err := backdata.ByID(guarantee.ID())
		if err == mempool.ErrEntityNotFound {
			completeBlock.CompleteCollections[guarantee.ID()] = &execution.CompleteCollection{
				Guarantee:    guarantee,
				Transactions: nil,
			}
			err := backdata.Add(&blockByCollection{
				CollectionID:  guarantee.ID(),
				CompleteBlock: completeBlock,
			})
			if err != nil {
				return fmt.Errorf("cannot save collection-block mapping: %w", err)
			}

			e.log.Debug().
				Hex("block_id", logging.Entity(completeBlock.Block)).
				Hex("collection_id", logging.ID(guarantee.ID())).
				Msg("requesting collection")

			err = e.collectionConduit.Submit(&messages.CollectionRequest{ID: guarantee.ID()}, collectionIdentifiers...)
			if err != nil {
				// TODO - this should be handled, maybe retried or put into some form of a queue
				e.log.Err(err).Msg("cannot submit collection requests")
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot get an item from mempool: %w", err)
		}
		if maybeBlockByCollection.ID() != completeBlock.Block.ID() {
			// Should not happen in MVP
			return fmt.Errorf("received block with same collection alredy pointing to different block ")
		}
	}

	return nil
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

	return e.computationEngine.ExecuteScript(script, block, blockView)
}

func (e *Engine) handleComputationResult(result *execution.ComputationResult, startState flow.StateCommitment) (flow.StateCommitment, error) {

	e.log.Debug().
		Hex("block_id", logging.ID(result.CompleteBlock.Block.ID())).
		Msg("received computationEngine result")

	chunks := make([]*flow.Chunk, len(result.StateViews))

	var endState flow.StateCommitment = startState

	for i, view := range result.StateViews {
		// TODO - Should the deltas be applied to a particular state?
		// Not important now, but might become important once we produce proofs
		var err error
		endState, err = e.execState.CommitDelta(view.Delta())
		if err != nil {
			return nil, fmt.Errorf("failed to apply chunk delta: %w", err)
		}
		//
		chunk := generateChunk(i, startState, endState)
		//
		chunkHeader := generateChunkHeader(chunk, view.Reads())
		//
		err = e.execState.PersistChunkHeader(chunkHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to save chunk header: %w", err)
		}
		//
		chunks[i] = chunk
		startState = endState
	}

	executionResult, err := e.generateExecutionResultForBlock(result.CompleteBlock, chunks, endState)
	if err != nil {
		return nil, fmt.Errorf("could not generate computationEngine result: %w", err)
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: *executionResult,
		// TODO: include SPoCKs
		Spocks: nil,
		// TODO: sign computationEngine receipt
		ExecutorSignature: nil,
		ExecutorID:        e.me.NodeID(),
	}

	err = e.execState.PersistStateCommitment(result.CompleteBlock.Block.ID(), endState)
	if err != nil {
		return nil, fmt.Errorf("failed to store state commitment: %w", err)
	}

	err = e.providerEngine.BroadcastExecutionReceipt(receipt)
	if err != nil {
		return nil, fmt.Errorf("could not send broadcast order: %w", err)
	}

	return endState, nil
}

// generateChunk creates a chunk from the provided computationEngine data.
func generateChunk(colIndex int, startState, endState flow.StateCommitment) *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(colIndex),
			StartState:      startState,
			// TODO: include event collection hash
			EventCollection: flow.ZeroID,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record number of txs
			NumberOfTransactions: 0,
		},
		Index:    0,
		EndState: endState,
	}
}

// generateChunkHeader creates a chunk header from the provided chunk and register IDs.
func generateChunkHeader(
	chunk *flow.Chunk,
	registerIDs []flow.RegisterID,
) *flow.ChunkHeader {

	return &flow.ChunkHeader{
		ChunkID:     chunk.ID(),
		StartState:  chunk.StartState,
		RegisterIDs: registerIDs,
	}
}

// generateExecutionResultForBlock creates a new computationEngine result for a block from
// the provided chunk results.
func (e *Engine) generateExecutionResultForBlock(
	block *execution.CompleteBlock,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(block.Block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get previous computationEngine result ID: %w", err)
	}

	er := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: previousErID,
			BlockID:          block.Block.ID(),
			FinalStateCommit: endState,
			Chunks:           chunks,
		},
	}

	err = e.execState.PersistExecutionResult(block.Block.ID(), *er)
	if err != nil {
		return nil, fmt.Errorf("could not persist computationEngine result: %w", err)
	}

	return er, nil
}
