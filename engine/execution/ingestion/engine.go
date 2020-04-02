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
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/mempool/queue"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	me                 module.Local
	state              protocol.State
	conduit            network.Conduit
	collectionConduit  network.Conduit
	blocks             storage.Blocks
	payloads           storage.Payloads
	collections        storage.Collections
	events             storage.Events
	computationManager computation.ComputationManager
	providerEngine     provider.ProviderEngine
	mempool            *Mempool
	execState          state.ExecutionState
	wg                 sync.WaitGroup
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	state protocol.State,
	blocks storage.Blocks,
	payloads storage.Payloads,
	collections storage.Collections,
	events storage.Events,
	executionEngine computation.ComputationManager,
	providerEngine provider.ProviderEngine,
	execState state.ExecutionState,
) (*Engine, error) {
	log := logger.With().Str("engine", "blocks").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:               engine.NewUnit(),
		log:                log,
		me:                 me,
		state:              state,
		blocks:             blocks,
		payloads:           payloads,
		collections:        collections,
		events:             events,
		computationManager: executionEngine,
		providerEngine:     providerEngine,
		mempool:            mempool,
		execState:          execState,
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

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*entity.CompleteCollection),
	}

	err = e.mempool.Run(func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueue *stdmap.QueuesBackdata, orphanQueue *stdmap.QueuesBackdata) error {

		err := e.sendCollectionsRequest(executableBlock, blockByCollection)
		if err != nil {
			return fmt.Errorf("cannot send collection requests: %w", err)
		}

		// if block fits into execution queue, that's it
		if queue, added := tryEnqueue(executableBlock, executionQueue); added {
			e.tryRequeueOrphans(executableBlock, queue, orphanQueue)
			return nil
		}

		// if block fits into orphan queues
		if queue, added := tryEnqueue(executableBlock, orphanQueue); added {
			e.tryRequeueOrphans(executableBlock, queue, orphanQueue)
			return nil
		}

		stateCommitment, err := e.execState.StateCommitmentByBlockID(block.ParentID)
		// if state commitment doesn't exist and there are no known blocks which will produce
		// it soon (execution queue) that we save it as orphaned
		if err == storage.ErrNotFound {
			_, err := enqueue(executableBlock, orphanQueue)
			if err != nil {
				panic(fmt.Sprintf("cannot add orphaned block: %s", err))
			}
			return nil
		}
		// any other error while accessing storage - panic
		if err != nil {
			panic(fmt.Sprintf("unexpected error while accessing storage, shutting down: %v", err))
		}

		executableBlock.StartState = stateCommitment
		newQueue, err := enqueue(executableBlock, executionQueue)
		if err != nil {
			panic(fmt.Sprintf("cannot enqueue block for execution: %s", err))
		}

		e.tryRequeueOrphans(executableBlock, newQueue, orphanQueue)

		// If the block was empty
		if executableBlock.IsComplete() {
			e.wg.Add(1)
			go e.executeBlock(executableBlock)
		}

		return nil
	})

	return err
}

// tryRequeueOrphans tries to put orphaned queue into execution queue after a new block has been added
func (e *Engine) tryRequeueOrphans(executableBlock *entity.ExecutableBlock, targetQueue *queue.Queue, potentialQueues *stdmap.QueuesBackdata) {
	for _, queue := range potentialQueues.All() {
		// only need to check for heads, as all children has parent already
		// there might be many queues sharing a parent
		if queue.Head.ExecutableBlock.Block.ParentID == executableBlock.Block.ID() {
			err := targetQueue.Attach(queue)
			// shouldn't happen
			if err != nil {
				panic(fmt.Sprintf("internal error while joining queues"))
			}
			potentialQueues.Rem(queue.ID())
		}
	}
}

func (e *Engine) executeBlock(executableBlock *entity.ExecutableBlock) {
	defer e.wg.Done()

	view := e.execState.NewView(executableBlock.StartState)
	e.log.Info().
		Hex("block_id", logging.Entity(executableBlock.Block)).
		Msg("executing block")

	computationResult, err := e.computationManager.ComputeBlock(executableBlock, view)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while computing block")
		return
	}

	finalState, err := e.handleComputationResult(computationResult, executableBlock.StartState)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while handing computation results")
		return
	}

	err = e.mempool.ExecutionQueue.Run(func(executionQueues *stdmap.QueuesBackdata) error {
		executionQueue, err := executionQueues.ByID(executableBlock.Block.ID())
		if err != nil {
			return fmt.Errorf("fatal error - executed block not present in execution queue: %w", err)
		}
		_, newQueues := executionQueue.Dismount()
		for _, queue := range newQueues {
			queue.Head.ExecutableBlock.StartState = finalState
			err := executionQueues.Add(queue)
			if err != nil {
				return fmt.Errorf("fatal error cannot add children block to execution queue: %w", err)
			}
			if queue.Head.ExecutableBlock.IsComplete() {
				e.wg.Add(1)
				go e.executeBlock(queue.Head.ExecutableBlock)
			}
		}
		executionQueues.Rem(executableBlock.Block.ID())
		return nil
	})

	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while requeueing blocks after execution")
	}
}

func (e *Engine) handleCollectionResponse(response *messages.CollectionResponse) error {

	collection := response.Collection

	e.log.Debug().
		Hex("collection_id", logging.Entity(collection)).
		Msg("received collection")

	collID := collection.ID()

	return e.mempool.BlockByCollection.Run(func(backdata *stdmap.BlockByCollectionBackdata) error {
		blockByCollectionId, err := backdata.ByID(collID)
		if err != nil {
			return err
		}
		executableBlock := blockByCollectionId.ExecutableBlock

		completeCollection, ok := executableBlock.CompleteCollections[collID]
		if !ok {
			return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
		}
		// already received transactions for this collection
		// TODO - check if data stored is the same
		if completeCollection.Transactions != nil {
			return nil
		}

		completeCollection.Transactions = collection.Transactions
		if executableBlock.HasAllTransactions() {
			e.clearCollectionsCache(executableBlock, backdata)
		}

		if executableBlock.IsComplete() {
			e.wg.Add(1)
			go e.executeBlock(executableBlock)
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

func (e *Engine) clearCollectionsCache(block *entity.ExecutableBlock, backdata *stdmap.BlockByCollectionBackdata) {
	for _, collection := range block.Block.Guarantees {
		backdata.Rem(collection.ID())
	}
}

// tryEnqueue checks if a block fits somewhere into the already existing queues, and puts it there is so
func tryEnqueue(executableBlock *entity.ExecutableBlock, queues *stdmap.QueuesBackdata) (*queue.Queue, bool) {
	for _, queue := range queues.All() {
		if queue.TryAdd(executableBlock) {
			return queue, true
		}
	}
	return nil, false
}

func newQueue(executableBlock *entity.ExecutableBlock, queues *stdmap.QueuesBackdata) (*queue.Queue, error) {
	q := queue.NewQueue(executableBlock)
	return q, queues.Add(q)
}

// enqueue inserts block into matching queue or creates a new one
func enqueue(executableBlock *entity.ExecutableBlock, queues *stdmap.QueuesBackdata) (*queue.Queue, error) {
	for _, queue := range queues.All() {
		if queue.TryAdd(executableBlock) {
			return queue, nil
		}
	}
	return newQueue(executableBlock, queues)
}

func (e *Engine) sendCollectionsRequest(executableBlock *entity.ExecutableBlock, backdata *stdmap.BlockByCollectionBackdata) error {

	collectionIdentifiers, err := e.findCollectionNodes()
	if err != nil {
		return err
	}

	for _, guarantee := range executableBlock.Block.Guarantees {
		maybeBlockByCollection, err := backdata.ByID(guarantee.ID())
		if err == mempool.ErrEntityNotFound {
			executableBlock.CompleteCollections[guarantee.ID()] = &entity.CompleteCollection{
				Guarantee:    guarantee,
				Transactions: nil,
			}
			err := backdata.Add(&entity.BlockByCollection{
				CollectionID:    guarantee.ID(),
				ExecutableBlock: executableBlock,
			})
			if err != nil {
				return fmt.Errorf("cannot save collection-block mapping: %w", err)
			}

			e.log.Debug().
				Hex("block_id", logging.Entity(executableBlock.Block)).
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
		if maybeBlockByCollection.ID() != executableBlock.Block.ID() {
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

	return e.computationManager.ExecuteScript(script, block, blockView)
}

func (e *Engine) handleComputationResult(result *execution.ComputationResult, startState flow.StateCommitment) (flow.StateCommitment, error) {

	e.log.Debug().
		Hex("block_id", logging.ID(result.ExecutableBlock.Block.ID())).
		Msg("received computation result")

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

		// chunkDataPack
		allRegisters := view.AllRegisters()
		values, proofs, err := e.execState.GetRegistersWithProofs(chunk.StartState, allRegisters)

		if err != nil {
			return nil, fmt.Errorf("error reading registers with proofs for chunk number [%v] of block [%x] ", i, result.ExecutableBlock.Block.ID())
		}

		chdp := generateChunkDataPack(chunk, allRegisters, values, proofs)
		err = e.execState.PersistChunkDataPack(chdp)
		if err != nil {
			return nil, fmt.Errorf("failed to save chunk data pack: %w", err)
		}
		//
		chunks[i] = chunk
		startState = endState
	}

	executionResult, err := e.generateExecutionResultForBlock(result.ExecutableBlock, chunks, endState)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution result: %w", err)
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: *executionResult,
		// TODO: include SPoCKs
		Spocks: nil,
		// TODO: sign execution receipt
		ExecutorSignature: nil,
		ExecutorID:        e.me.NodeID(),
	}

	err = e.execState.PersistStateCommitment(result.ExecutableBlock.Block.ID(), endState)
	if err != nil {
		return nil, fmt.Errorf("failed to store state commitment: %w", err)
	}

	if len(result.Events) > 0 {
		err = e.events.Store(result.ExecutableBlock.Block.ID(), result.Events)
		if err != nil {
			return nil, fmt.Errorf("failed to store events: %w", err)
		}
	}

	err = e.providerEngine.BroadcastExecutionReceipt(receipt)
	if err != nil {
		return nil, fmt.Errorf("could not send broadcast order: %w", err)
	}

	return endState, nil
}

// generateChunk creates a chunk from the provided computation data.
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

// generateExecutionResultForBlock creates new ExecutionResult for a block from
// the provided chunk results.
func (e *Engine) generateExecutionResultForBlock(
	block *entity.ExecutableBlock,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(block.Block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get previous execution result ID: %w", err)
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
		return nil, fmt.Errorf("could not persist execution result: %w", err)
	}

	return er, nil
}

// generateChunkDataPack creates a chunk data pack
func generateChunkDataPack(
	chunk *flow.Chunk,
	registers []flow.RegisterID,
	values []flow.RegisterValue,
	proofs []flow.StorageProof,
) *flow.ChunkDataPack {
	regTs := make([]flow.RegisterTouch, len(registers))
	for i, reg := range registers {
		regTs[i] = flow.RegisterTouch{RegisterID: reg,
			Value: values[i],
			Proof: proofs[i],
		}
	}
	return &flow.ChunkDataPack{
		ChunkID:         chunk.ID(),
		StartState:      chunk.StartState,
		RegisterTouches: regTs,
	}
}
