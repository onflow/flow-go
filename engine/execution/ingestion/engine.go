package ingestion

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation"
	"github.com/dapperlabs/flow-go/engine/execution/provider"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	executionSync "github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/engine/execution/utils"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/mempool/queue"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	me                 module.Local
	state              protocol.State
	receiptHasher      hash.Hasher // used as hasher to sign the execution receipt
	conduit            network.Conduit
	collectionConduit  network.Conduit
	syncConduit        network.Conduit
	blocks             storage.Blocks
	payloads           storage.Payloads
	collections        storage.Collections
	events             storage.Events
	computationManager computation.ComputationManager
	providerEngine     provider.ProviderEngine
	mempool            *Mempool
	execState          state.ExecutionState
	wg                 sync.WaitGroup
	syncWg             sync.WaitGroup
	syncModeThreshold  uint64 //how many consecutive orphaned blocks trigger sync
	syncInProgress     *atomic.Bool
	syncTargetBlockID  atomic.Value
	stateSync          executionSync.StateSynchronizer
	mc                 module.Metrics
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
	syncThreshold uint64,
	mc module.Metrics,
) (*Engine, error) {
	log := logger.With().Str("engine", "blocks").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:               engine.NewUnit(),
		log:                log,
		me:                 me,
		state:              state,
		receiptHasher:      utils.NewExecutionReceiptHasher(),
		blocks:             blocks,
		payloads:           payloads,
		collections:        collections,
		events:             events,
		computationManager: executionEngine,
		providerEngine:     providerEngine,
		mempool:            mempool,
		execState:          execState,
		syncModeThreshold:  syncThreshold,
		syncInProgress:     atomic.NewBool(false),
		stateSync:          executionSync.NewStateSynchronizer(execState),
		mc:                 mc,
	}

	con, err := net.Register(engine.BlockProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	collConduit, err := net.Register(engine.CollectionProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register collection provider engine")
	}

	syncConduit, err := net.Register(engine.ExecutionSync, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution sync engine")
	}

	eng.conduit = con
	eng.collectionConduit = collConduit
	eng.syncConduit = syncConduit

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
		e.Wait()
	})
}

func (e *Engine) Wait() {
	e.wg.Wait()     //wait for block execution
	e.syncWg.Wait() // wait for sync
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {

	return e.unit.Do(func() error {
		var err error
		switch v := event.(type) {
		case *messages.BlockProposal:
			err = e.handleBlockProposal(v)
		case *messages.CollectionResponse:
			err = e.handleCollectionResponse(v)
		case *messages.ExecutionStateDelta:
			err = e.handleExecutionStateDelta(v, originID)
		case *messages.ExecutionStateSyncRequest:
			return e.onExecutionStateSyncRequest(originID, v)
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

func (e *Engine) handleBlockProposal(proposal *messages.BlockProposal) error {

	block := &flow.Block{
		Header:  *proposal.Header,
		Payload: *proposal.Payload,
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Uint64("block_view", block.View).
		Msg("received block")

	e.mc.StartBlockReceivedToExecuted(block.ID())

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*entity.CompleteCollection),
	}

	return e.mempool.Run(func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueues *stdmap.QueuesBackdata, orphanQueues *stdmap.QueuesBackdata) error {

		// synchronize DB writing to avoid tx conflicts with multiple blocks arriving fast
		err := e.blocks.Store(block)
		if err != nil {
			return fmt.Errorf("could not store block: %w", err)
		}

		// if block fits into execution queue, that's it
		if queue, added := tryEnqueue(executableBlock, executionQueues); added {
			e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("added block to existing execution queue")
			e.tryRequeueOrphans(executableBlock, queue, orphanQueues)
			return nil
		}

		// if block fits into orphan queues
		if queue, added := tryEnqueue(executableBlock, orphanQueues); added {
			e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("added block to existing orphan queue")
			e.tryRequeueOrphans(executableBlock, queue, orphanQueues)
			// this is only queue which grew and could trigger threshold
			if !e.syncInProgress.Load() && queue.Height() >= e.syncModeThreshold {
				e.syncInProgress.Store(true)
				// Start sync mode - initializing would require DB operation and will stop processing blocks here
				// which is exactly what we want
				e.StartSync(queue.Head.Item.(*entity.ExecutableBlock))
			}
			return nil
		}

		stateCommitment, err := e.execState.StateCommitmentByBlockID(block.ParentID)
		// if state commitment doesn't exist and there are no known blocks which will produce
		// it soon (execution queue) that we save it as orphaned
		if err == storage.ErrNotFound {
			queue, err := enqueue(executableBlock, orphanQueues)
			if err != nil {
				panic(fmt.Sprintf("cannot add orphaned block: %s", err))
			}
			e.tryRequeueOrphans(executableBlock, queue, orphanQueues)
			e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("added block to new orphan queue")
			// special case when sync threshold is zero
			if queue.Height() >= e.syncModeThreshold && !e.syncInProgress.Load() {
				e.syncInProgress.Store(true)
				// Start sync mode - initializing would require DB operation and will stop processing blocks here
				// which is exactly what we want
				e.StartSync(queue.Head.Item.(*entity.ExecutableBlock))
			}
			return nil
		}
		// any other error while accessing storage - panic
		if err != nil {
			panic(fmt.Sprintf("unexpected error while accessing storage, shutting down: %v", err))
		}

		//if block has state commitment, it has all parents blocks
		err = e.sendCollectionsRequest(executableBlock, blockByCollection)
		if err != nil {
			return fmt.Errorf("cannot send collection requests: %w", err)
		}

		executableBlock.StartState = stateCommitment
		newQueue, err := enqueue(executableBlock, executionQueues) // TODO - redundant? - should always produce new queue (otherwise it would be enqueued at the beginning)
		if err != nil {
			panic(fmt.Sprintf("cannot enqueue block for execution: %s", err))
		}
		e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("added block to execution queue")

		e.tryRequeueOrphans(executableBlock, newQueue, orphanQueues)

		// If the block was empty
		if executableBlock.IsComplete() {
			e.wg.Add(1)
			go e.executeBlock(executableBlock)
		}

		return nil
	})
}

// tryRequeueOrphans tries to put orphaned queue into other queues after a new block has been added
func (e *Engine) tryRequeueOrphans(blockify queue.Blockify, targetQueue *queue.Queue, potentialQueues *stdmap.QueuesBackdata) {
	for _, queue := range potentialQueues.All() {
		// only need to check for heads, as all children has parent already
		// there might be many queues sharing a parent
		if queue.Head.Item.ParentID() == blockify.ID() {
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

	e.mc.FinishBlockReceivedToExecuted(executableBlock.Block.ID())
	e.mc.ExecutionGasUsedPerBlock(computationResult.GasUsed)
	e.mc.ExecutionStateReadsPerBlock(computationResult.StateReads)

	finalState, err := e.handleComputationResult(computationResult, executableBlock.StartState)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while handing computation results")
		return
	}

	diskTotal, err := e.execState.Size()
	if err != nil {
		e.log.Err(err).Msg("could not get execution state disk size")
	}
	e.mc.ExecutionStateStorageDiskTotal(diskTotal)
	e.mc.ExecutionStorageStateCommitment(int64(len(finalState)))

	err = e.mempool.Run(func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueues *stdmap.QueuesBackdata, _ *stdmap.QueuesBackdata) error {

		executionQueue, err := executionQueues.ByID(executableBlock.Block.ID())
		if err != nil {
			return fmt.Errorf("fatal error - executed block not present in execution queue: %w", err)
		}
		_, newQueues := executionQueue.Dismount()
		for _, queue := range newQueues {
			newExecutableBlock := queue.Head.Item.(*entity.ExecutableBlock)
			newExecutableBlock.StartState = finalState

			err = e.sendCollectionsRequest(newExecutableBlock, blockByCollection)
			if err != nil {
				return fmt.Errorf("cannot send collection requests: %w", err)
			}

			err := executionQueues.Add(queue)
			if err != nil {
				return fmt.Errorf("fatal error cannot add children block to execution queue: %w", err)
			}
			if newExecutableBlock.IsComplete() {
				e.wg.Add(1)
				go e.executeBlock(newExecutableBlock)
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
	e.log.Info().
		Hex("block_id", logging.Entity(executableBlock.Block)).
		Hex("final_state", finalState).
		Msg("executing block")
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

			e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("block complete - executing")

			e.wg.Add(1)
			go e.executeBlock(executableBlock)
		}

		return nil
	})
}

func (e *Engine) findCollectionNodesForGuarantee(blockID flow.Identifier, guarantee *flow.CollectionGuarantee) ([]flow.Identifier, error) {

	filter := filter.And(
		filter.HasRole(flow.RoleCollection),
		filter.HasNodeID(guarantee.SignerIDs...))

	identities, err := e.state.AtBlockID(blockID).Identities(filter)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities (%s): %w", blockID, err)
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

func (e *Engine) onExecutionStateSyncRequest(originID flow.Identifier, req *messages.ExecutionStateSyncRequest) error {
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("current_block_id", logging.ID(req.CurrentBlockID)).
		Hex("target_block_id", logging.ID(req.TargetBlockID)).
		Msg("received execution state synchronization request")

	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid role for requesting state synchronization: %s", id.Role)
	}

	err = e.stateSync.DeltaRange(
		req.CurrentBlockID,
		req.TargetBlockID,
		func(delta *messages.ExecutionStateDelta) error {
			e.log.Debug().
				Hex("origin_id", logging.ID(originID)).
				Hex("block_id", logging.ID(delta.Block.ID())).
				Msg("sending block delta")

			err := e.syncConduit.Submit(delta, originID)
			if err != nil {
				return fmt.Errorf("could not submit block delta: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to process block range: %w", err)
	}

	return nil
}

func (e *Engine) clearCollectionsCache(block *entity.ExecutableBlock, backdata *stdmap.BlockByCollectionBackdata) {
	for _, collection := range block.Block.Guarantees {
		backdata.Rem(collection.ID())
	}
}

// tryEnqueue checks if a block fits somewhere into the already existing queues, and puts it there is so
func tryEnqueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (*queue.Queue, bool) {
	for _, queue := range queues.All() {
		if queue.TryAdd(blockify) {
			return queue, true
		}
	}
	return nil, false
}

func newQueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (*queue.Queue, error) {
	q := queue.NewQueue(blockify)
	return q, queues.Add(q)
}

// enqueue inserts block into matching queue or creates a new one
func enqueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (*queue.Queue, error) {
	for _, queue := range queues.All() {
		if queue.TryAdd(blockify) {
			return queue, nil
		}
	}
	return newQueue(blockify, queues)
}

func (e *Engine) sendCollectionsRequest(executableBlock *entity.ExecutableBlock, backdata *stdmap.BlockByCollectionBackdata) error {

	for _, guarantee := range executableBlock.Block.Guarantees {

		// TODO - Once collection can map to multiple blocks
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

			collectionGuaranteesIdentifiers, err := e.findCollectionNodesForGuarantee(executableBlock.Block.ID(), guarantee)
			if err != nil {
				return err
			}

			e.log.Debug().
				Hex("block_id", logging.Entity(executableBlock.Block)).
				Hex("collection_id", logging.ID(guarantee.ID())).
				Msg("requesting collection")

			err = e.collectionConduit.Submit(&messages.CollectionRequest{ID: guarantee.ID()}, collectionGuaranteesIdentifiers...)
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
			// Should not happen in MVP, but see TODO at beggining of the function
			return fmt.Errorf("received block with same collection alredy pointing to different block ")
		}
	}

	return nil
}

func (e *Engine) ExecuteScriptAtBlockID(script []byte, blockID flow.Identifier) ([]byte, error) {

	stateCommit, err := e.execState.StateCommitmentByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}
	block, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	blockView := e.execState.NewView(stateCommit)

	return e.computationManager.ExecuteScript(script, block, blockView)
}

func (e *Engine) GetAccount(address flow.Address, blockID flow.Identifier) (*flow.Account, error) {

	stateCommit, err := e.execState.StateCommitmentByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}
	block, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	blockView := e.execState.NewView(stateCommit)

	return e.computationManager.GetAccount(address, block, blockView)
}

func (e *Engine) handleComputationResult(result *execution.ComputationResult, startState flow.StateCommitment) (flow.StateCommitment, error) {

	e.log.Debug().
		Hex("block_id", logging.ID(result.ExecutableBlock.Block.ID())).
		Msg("received computation result")

	receipt, err := e.saveExecutionResults(result.ExecutableBlock.Block, result.StateSnapshots, result.Events, startState)
	if err != nil {
		return nil, err
	}

	err = e.providerEngine.BroadcastExecutionReceipt(receipt)
	if err != nil {
		return nil, fmt.Errorf("could not send broadcast order: %w", err)
	}

	return receipt.ExecutionResult.FinalStateCommit, nil
}

func (e *Engine) saveExecutionResults(block *flow.Block, stateInteractions []*delta.Snapshot, events []flow.Event, startState flow.StateCommitment) (*flow.ExecutionReceipt, error) {

	originalState := startState

	err := e.execState.PersistStateInteractions(block.ID(), stateInteractions)
	if err != nil {
		return nil, err
	}

	chunks := make([]*flow.Chunk, len(stateInteractions))

	// TODO check current state root == startState
	var endState flow.StateCommitment = startState

	for i, view := range stateInteractions {
		// TODO - Deltas should be applied to a particular state
		var err error
		endState, err = e.execState.CommitDelta(view.Delta, startState)
		if err != nil {
			return nil, fmt.Errorf("failed to apply chunk delta: %w", err)
		}
		//
		chunk := generateChunk(i, startState, endState)

		// chunkDataPack
		allRegisters := view.RegisterTouches()
		values, proofs, err := e.execState.GetRegistersWithProofs(chunk.StartState, allRegisters)

		if err != nil {
			return nil, fmt.Errorf("error reading registers with proofs for chunk number [%v] of block [%x] ", i, block.ID())
		}

		chdp := generateChunkDataPack(chunk, allRegisters, values, proofs)
		err = e.execState.PersistChunkDataPack(chdp)
		if err != nil {
			return nil, fmt.Errorf("failed to save chunk data pack: %w", err)
		}
		// TODO use view.SpockSecret() as an input to spock generator
		chunks[i] = chunk
		startState = endState
	}

	err = e.execState.PersistStateCommitment(block.ID(), endState)
	if err != nil {
		return nil, fmt.Errorf("failed to store state commitment: %w", err)
	}

	err = e.execState.UpdateHighestExecutedBlockIfHigher(&block.Header)
	if err != nil {
		return nil, fmt.Errorf("failed to update highest executed block: %w", err)
	}

	executionResult, err := e.generateExecutionResultForBlock(block, chunks, endState)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution result: %w", err)
	}

	receipt, err := e.generateExecutionReceipt(executionResult)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution receipt: %w", err)
	}

	if len(events) > 0 {
		err = e.events.Store(block.ID(), events)
		if err != nil {
			return nil, fmt.Errorf("failed to store events: %w", err)
		}
	}

	e.log.Debug().Hex("block_id", logging.Entity(block)).Hex("start_state", originalState).Hex("final_state", endState).Msg("saved computation results")

	err = e.providerEngine.BroadcastExecutionReceipt(receipt)
	if err != nil {
		return nil, fmt.Errorf("could not send broadcast order: %w", err)
	}

	return receipt, nil
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

// generateExecutionResultForBlock creates new ExecutionResult for a block from
// the provided chunk results.
func (e *Engine) generateExecutionResultForBlock(
	block *flow.Block,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get previous execution result ID: %w", err)
	}

	er := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: previousErID,
			BlockID:          block.ID(),
			FinalStateCommit: endState,
			Chunks:           chunks,
		},
	}

	err = e.execState.PersistExecutionResult(block.ID(), *er)
	if err != nil {
		return nil, fmt.Errorf("could not persist execution result: %w", err)
	}

	return er, nil
}

func (e *Engine) generateExecutionReceipt(result *flow.ExecutionResult) (*flow.ExecutionReceipt, error) {
	receipt := &flow.ExecutionReceipt{
		ExecutionResult:   *result,
		Spocks:            nil, // TODO: include SPoCKs
		ExecutorSignature: crypto.Signature{},
		ExecutorID:        e.me.NodeID(),
	}

	// generates a signature over the execution result
	b, err := encoding.DefaultEncoder.Encode(receipt.Body())
	if err != nil {
		return nil, fmt.Errorf("could not encode execution result: %w", err)
	}
	sig, err := e.me.Sign(b, e.receiptHasher)
	if err != nil {
		return nil, fmt.Errorf("could not sign execution result: %w", err)
	}

	receipt.ExecutorSignature = sig

	return receipt, nil
}

func (e *Engine) StartSync(firstKnown *entity.ExecutableBlock) {
	// find latest finalized block with state commitment
	// this way we maximise chance of path existing between it and fresh one
	// TODO - this doesn't make sense if we treat every block as finalized (MVP)

	targetBlockID := firstKnown.Block.ParentID

	e.syncTargetBlockID.Store(targetBlockID)

	e.log.Info().Msg("starting state synchronisation")

	lastExecutedHeight, lastExecutedBlockID, err := e.execState.GetHighestExecutedBlockID()
	if err != nil {
		e.log.Fatal().Err(err).Msg("error while starting sync - cannot find highest executed block")
	}

	e.log.Debug().Msgf("sync from height %d to height %d", lastExecutedHeight, firstKnown.Block.Height-1)

	otherNodes, err := e.state.Final().Identities(filter.And(filter.HasRole(flow.RoleExecution), e.me.NotMeFilter()))
	if err != nil {
		e.log.Fatal().Err(err).Msg("error while finding other execution nodes identities")
		return
	}

	if len(otherNodes) < 1 {
		e.log.Fatal().Err(err).Msg("no other execution nodes to sync from")
	}

	// select other node at random
	// TODO - protocol which surveys other nodes for state
	// TODO - ability to sync from multiple servers
	otherNodeIdentity := otherNodes[rand.Intn(len(otherNodes))]

	e.log.Debug().Hex("target_node", logging.Entity(otherNodeIdentity)).Msg("requesting sync from node")

	err = e.syncConduit.Submit(&messages.ExecutionStateSyncRequest{
		CurrentBlockID: lastExecutedBlockID,
		TargetBlockID:  targetBlockID,
	}, otherNodeIdentity.NodeID)

	if err != nil {
		e.log.Fatal().Err(err).Str("target_node_id", otherNodeIdentity.NodeID.String()).Msg("error while requesting state sync from other node")
	}
}

func (e *Engine) handleExecutionStateDelta(executionStateDelta *messages.ExecutionStateDelta, originID flow.Identifier) error {

	e.log.Debug().Hex("block_id", logging.Entity(executionStateDelta.Block)).Msg("received sync delta")

	return e.mempool.SyncQueues.Run(func(backdata *stdmap.QueuesBackdata) error {

		//try enqueue
		if queue, added := tryEnqueue(executionStateDelta, backdata); added {
			e.log.Debug().Hex("block_id", logging.Entity(executionStateDelta.Block)).Msg("added block to existing orphan queue")
			e.tryRequeueOrphans(executionStateDelta, queue, backdata)
			return nil
		}

		stateCommitment, err := e.execState.StateCommitmentByBlockID(executionStateDelta.ParentID())
		// if state commitment doesn't exist and there are no known deltas which will produce
		// it soon (sync queue) that we save it as orphaned
		if err == storage.ErrNotFound {
			_, err := enqueue(executionStateDelta, backdata)
			if err != nil {
				panic(fmt.Sprintf("cannot create new queue for sync delta: %s", err))
			}
			return nil
		}
		if err != nil {
			panic(fmt.Sprintf("unexpected error while accessing storage for sync deltas, shutting down: %v", err))
		}

		newQueue, err := enqueue(executionStateDelta, backdata) // TODO - redundant? - should always produce new queue (otherwise it would be enqueued at the beginning)
		if err != nil {
			panic(fmt.Sprintf("cannot enqueue sync delta: %s", err))
		}

		e.tryRequeueOrphans(executionStateDelta, newQueue, backdata)

		if !bytes.Equal(stateCommitment, executionStateDelta.StartState) {
			return fmt.Errorf("internal incosistency with delta - state commitment for parent retirieved from DB different from start state in delta! ")
		}

		e.syncWg.Add(1)
		go e.saveDelta(executionStateDelta)

		return nil
	})
}

func (e *Engine) saveDelta(executionStateDelta *messages.ExecutionStateDelta) {

	defer e.syncWg.Done()

	// synchronize DB writing to avoid tx conflicts with multiple blocks arriving fast
	err := e.blocks.Store(executionStateDelta.Block)
	if err != nil {
		e.log.Fatal().Hex("block_id", logging.Entity(executionStateDelta.Block)).
			Err(err).Msg("could  not store block from delta")
	}

	//TODO - validate state sync, reject invalid messages, change provider
	executionReceipt, err := e.saveExecutionResults(executionStateDelta.Block, executionStateDelta.StateInteractions, executionStateDelta.Events, executionStateDelta.StartState)
	if err != nil {
		e.log.Fatal().Hex("block_id", logging.Entity(executionStateDelta.Block)).Err(err).Msg("fatal error while processing sync message")
	}

	if !bytes.Equal(executionReceipt.ExecutionResult.FinalStateCommit, executionStateDelta.EndState) {
		e.log.Fatal().Hex("block_id", logging.Entity(executionStateDelta.Block)).
			Hex("saved_state", executionReceipt.ExecutionResult.FinalStateCommit).
			Hex("delta_end_state", executionStateDelta.EndState).
			Hex("delta_start_state", executionStateDelta.StartState).
			Err(err).Msg("processing sync message produced unexpected state commitment")
	}

	targetBlockID := e.syncTargetBlockID.Load().(flow.Identifier)

	// last block was saved
	if targetBlockID == executionStateDelta.Block.ID() {

		err = e.mempool.Run(func(blockByCollection *stdmap.BlockByCollectionBackdata, executionQueues *stdmap.QueuesBackdata, orphanQueues *stdmap.QueuesBackdata) error {
			var syncedQueue *queue.Queue
			hadQueue := false
			for _, q := range orphanQueues.All() {
				if q.Head.Item.(*entity.ExecutableBlock).Block.ParentID == targetBlockID {
					syncedQueue = q
					hadQueue = true
					break
				}
			}
			if !hadQueue {
				panic(fmt.Sprintf("orphan queues do not contain final block ID (%s)", targetBlockID))
			}
			orphanQueues.Rem(syncedQueue.ID())

			//if the state we generated from applying this is not equal to EndState we would have panicked earlier
			executableBlock := syncedQueue.Head.Item.(*entity.ExecutableBlock)
			executableBlock.StartState = executionStateDelta.EndState

			err = e.sendCollectionsRequest(executableBlock, blockByCollection)
			if err != nil {
				return fmt.Errorf("cannot send collection requests: %w", err)
			}

			if executableBlock.IsComplete() {
				err = executionQueues.Add(syncedQueue)
				if err != nil {
					panic(fmt.Sprintf("cannot add queue to execution queues"))
				}

				e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("block complete - executing")

				e.wg.Add(1)
				go e.executeBlock(executableBlock)
			}

			return nil
		})

		if err != nil {
			e.log.Err(err).Hex("block_id", logging.Entity(executionStateDelta.Block)).Msg("error while processing final target sync block")
		}

		return
	}

	err = e.mempool.SyncQueues.Run(func(backdata *stdmap.QueuesBackdata) error {

		executionQueue, err := backdata.ByID(executionStateDelta.Block.ID())
		if err != nil {
			return fmt.Errorf("fatal error - synced delta not present in sync queue: %w", err)
		}
		_, newQueues := executionQueue.Dismount()
		for _, queue := range newQueues {
			if !bytes.Equal(queue.Head.Item.(*messages.ExecutionStateDelta).StartState, executionReceipt.ExecutionResult.FinalStateCommit) {
				return fmt.Errorf("internal incosistency with delta - state commitment for after applying delta different from start state of next one! ")
			}
			err := backdata.Add(queue)
			if err != nil {
				return fmt.Errorf("fatal error cannot add children block to sync queue: %w", err)
			}

			e.syncWg.Add(1)
			go e.saveDelta(queue.Head.Item.(*messages.ExecutionStateDelta))
		}
		backdata.Rem(executionStateDelta.Block.ID())
		return nil
	})

	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executionStateDelta.Block)).
			Msg("error while requeueing delta after saving")
	}

	e.log.Debug().Hex("block_id", logging.Entity(executionStateDelta.Block)).Msg("finished processing sync delta")
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
