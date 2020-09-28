package ingestion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	executionSync "github.com/onflow/flow-go/engine/execution/sync"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/model/events"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	me                 module.Local
	request            module.Requester // used to request collections
	state              protocol.State
	receiptHasher      hash.Hasher // used as hasher to sign the execution receipt
	blocks             storage.Blocks
	collections        storage.Collections
	events             storage.Events
	transactionResults storage.TransactionResults
	computationManager computation.ComputationManager
	providerEngine     provider.ProviderEngine
	mempool            *Mempool
	execState          state.ExecutionState
	wg                 sync.WaitGroup
	metrics            module.ExecutionMetrics
	tracer             module.Tracer
	extensiveLogging   bool
	spockHasher        hash.Hasher
	root               *flow.Header
	checkingSyncing    mutex.Lock
	syncThreshold      int
	syncHeight         uint64
	syncFilter         flow.IdentityFilter
	syncConduit        network.Conduit
	syncDeltas         mempool.Deltas
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	request module.Requester,
	state protocol.State,
	blocks storage.Blocks,
	collections storage.Collections,
	events storage.Events,
	transactionResults storage.TransactionResults,
	executionEngine computation.ComputationManager,
	providerEngine provider.ProviderEngine,
	execState state.ExecutionState,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	extLog bool,
	root *flow.Header,
	syncFilter flow.IdentityFilter,
) (*Engine, error) {
	log := logger.With().Str("engine", "ingestion").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:               engine.NewUnit(),
		log:                log,
		me:                 me,
		request:            request,
		state:              state,
		receiptHasher:      utils.NewExecutionReceiptHasher(),
		spockHasher:        utils.NewSPOCKHasher(),
		blocks:             blocks,
		collections:        collections,
		events:             events,
		transactionResults: transactionResults,
		computationManager: executionEngine,
		providerEngine:     providerEngine,
		mempool:            mempool,
		execState:          execState,
		metrics:            metrics,
		tracer:             tracer,
		extensiveLogging:   extLog,
		root:               root,
		syncFilter:         syncFilter,
	}

	// move to state syncing engine
	syncConduit, err := net.Register(engine.SyncExecution, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register execution blockSync engine: %w", err)
	}

	eng.syncConduit = syncConduit

	return &eng, nil
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

// Wait will wait until all the blocks that are currently being executing
// finish being executed.
func (e *Engine) Wait() {
	e.wg.Wait() // wait for block execution
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
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return fmt.Errorf("ingestion error does not process local events")
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	var err error

	switch resource := event.(type) {
	case *messages.ExecutionStateSyncRequest:
		err = e.handleStateSyncRequest(originID, resource)
	case *messages.ExecutionStateDelta:
		err = e.handleStateDeltaResponse(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	if err != nil {
		e.log.Debug().Err(err).Msg("engine could not process event successfully")
	}

	return nil
}

// OnBlockIncorporated handles the new incorporated blocks (blocks that
// have passed consensus validation) received from the consensus nodes
func (e *Engine) OnBlockIncorporated(b *model.Block) {
	newBlock, err := e.blocks.ByID(b.BlockID)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("could not get incorporated block(%v): %v", b.BlockID, err)
	}

	if newBlock.Header.View <= e.root.View {
		// 	ignore root block since it's different from other blocks:
		//  1) root block doesn't have parent execution result
		//  2) root block's result has been persistent already
		return
	}

	err = e.handleBlock(context.Background(), newBlock)
	if err != nil {
		e.log.Error().Err(err).Hex("block_id", logging.ID(b.BlockID)).Msg("failed to handle block proposal")
	}
}

// required to provide implementation as a consensus follwer engine
func (e *Engine) OnFinalizedBlock(block *model.Block)                {}
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {}

// Main handling

// handle block will process the incoming block.
// the block has passed the consensus validation.
func (e *Engine) handleBlock(ctx context.Context, block *flow.Block) error {

	e.metrics.StartBlockReceivedToExecuted(block.ID())

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*entity.CompleteCollection),
	}

	parentCommitment, err := e.execState.StateCommitmentByBlockID(ctx, block.Header.ParentID)

	// acquiring the lock so that there is one process modifying the queue
	return e.mempool.Run(
		func(
			blockByCollection *stdmap.BlockByCollectionBackdata,
			executionQueues *stdmap.QueuesBackdata,
		) error {
			// check if this block is a new block by checking if the execution queue has this block already
			queue, added := tryEnqueue(executableBlock, executionQueues)

			// if it's not a new block, then bail
			if !added {
				e.log.Debug().
					Hex("block_id", logging.Entity(executableBlock)).
					Msg("added block to existing execution queue")
				return nil
			}

			// whenever the queue grows, we need to check whether the state sync should be
			// triggered.
			// TODO: run in a different goroutine
			go e.checkStateSyncStart(queue)

			// check if this block has been executed already

			// check if we need to trigger state sync

			// check if a block is executable.
			// a block is executable if the following conditions are all true
			// 1) the parent state commitment is ready
			// 2) the collections for the block payload are ready
			// 3) TODO: the child block is ready for querying the randomness

			// check if the block's parent has been executed. (we can't execute the block if the parent has
			// not been executed yet)
			// check if there is a statecommitment for the parent block
			parentCommitment, err := e.execState.StateCommitmentByBlockID(ctx, block.Header.ParentID)

			// if we found the statecommitment for the parent block, then add it to the executable block.
			if err == nil {
				executableBlock.StartState = parentCommitment
			} else if !errors.Is(err, storage.ErrNotFound) {
				// if there is exception, then crash
				e.log.Fatal().Err(err).Msg("unexpected error while accessing storage, shutting down")
			}

			// check if we have all the collections for the block, and request them if there is missing.
			err = e.matchOrRequestCollections(executableBlock, blockByCollection)
			if err != nil {
				return fmt.Errorf("cannot send collection requests: %w", err)
			}

			// If the block was empty
			e.executeBlockIfComplete(executableBlock)

			return nil
		},
	)
}

// executeBlock will execute the block.
// When finish executing, it will check if the children becomes executable and execute them if yes.
func (e *Engine) executeBlock(ctx context.Context, executableBlock *entity.ExecutableBlock) {
	defer e.wg.Done()

	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.EXEExecuteBlock)
	defer span.Finish()

	view := e.execState.NewView(executableBlock.StartState)
	e.log.Info().
		Hex("block_id", logging.Entity(executableBlock)).
		Msg("executing block")

	computationResult, err := e.computationManager.ComputeBlock(ctx, executableBlock, view)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock)).
			Msg("error while computing block")
		return
	}

	e.metrics.FinishBlockReceivedToExecuted(executableBlock.ID())
	e.metrics.ExecutionGasUsedPerBlock(computationResult.GasUsed)
	e.metrics.ExecutionStateReadsPerBlock(computationResult.StateReads)

	finalState, err := e.handleComputationResult(ctx, computationResult, executableBlock.StartState)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock)).
			Msg("error while handing computation results")
		return
	}

	diskTotal, err := e.execState.DiskSize()
	if err != nil {
		e.log.Err(err).Msg("could not get execution state disk size")
	}

	e.metrics.ExecutionStateStorageDiskTotal(diskTotal)
	e.metrics.ExecutionStorageStateCommitment(int64(len(finalState)))

	e.log.Info().
		Hex("block_id", logging.Entity(executableBlock)).
		Uint64("block_height", executableBlock.Block.Header.Height).
		Hex("final_state", finalState).
		Msg("block executed")

	e.metrics.ExecutionLastExecutedBlockHeight(executableBlock.Block.Header.Height)

	err = e.onBlockExecuted(executableBlock, finalState)
	if err != nil {
		e.log.Err(err).Msg("failed in process block's children")
	}
}

// we've executed the block, now we need to check:
// 1. whether the state syncing can be turned off
// 2. whether its children can be executed
//   the executionQueues stores blocks as a tree:
//
//   10 <- 11 <- 12
//   	 ^-- 13
//   14 <- 15 <- 16
//
//   if block 10 is the one just executed, then we will remove it from the queue, and add
//   its children back, meaning the tree will become:
//
//   11 <- 12
//   13
//   14 <- 15 <- 16

func (e *Engine) onBlockExecuted(executedBlock *entity.ExecutableBlock, finalState flow.StateCommitment) error {

	e.checkStateSyncStop(executedBlock)

	err := e.mempool.Run(
		func(
			blockByCollection *stdmap.BlockByCollectionBackdata,
			executionQueues *stdmap.QueuesBackdata,
		) error {

			// find the block that was just executed
			executionQueue, exists := executionQueues.ByID(executedBlock.ID())
			if !exists {
				return fmt.Errorf("fatal error - executed block not present in execution queue")
			}

			// dismount the executed block and all its children
			_, newQueues := executionQueue.Dismount()

			// go through each children, add them back to the queue, and check
			// if the children is executable
			for _, queue := range newQueues {
				added := executionQueues.Add(queue)
				if !added {
					return fmt.Errorf("fatal error - child block already in execution queue")
				}

				// the parent block has been executed, update the StartState of
				// each child block.
				childBlock := queue.Head.Item.(*entity.ExecutableBlock)
				childBlock.StartState = finalState

				err := e.matchOrRequestCollections(childBlock, blockByCollection)
				if err != nil {
					return fmt.Errorf("cannot send collection requests: %w", err)
				}

				e.executeBlockIfComplete(childBlock)
			}

			// remove the executed block
			executionQueues.Rem(executedBlock.ID())

			return nil
		})

	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executedBlock)).
			Msg("error while requeueing blocks after execution")
	}

	return nil
}

// executeBlockIfComplete checks whether the block is ready to be executed.
// if yes, execute the block
func (e *Engine) executeBlockIfComplete(eb *entity.ExecutableBlock) bool {
	if !eb.HasStartState() {
		return false
	}

	// if the eb has parent statecommitment, and we have the delta for this block
	// then apply the delta
	// note the block ID is the delta's ID
	delta, found := e.syncDeltas.ByID(eb.Block.ID())
	if found {
		go e.applyStateDelta(delta)
		return true
	}

	// if don't have the delta, then check if everything is ready for executing
	// the block
	if eb.IsComplete() {
		e.log.Debug().
			Hex("block_id", logging.Entity(eb)).
			Msg("block complete, starting execution")

		if e.extensiveLogging {
			e.logExecutableBlock(eb)
		}

		e.wg.Add(1)
		go e.executeBlock(context.Background(), eb)
		return true
	}
	return false
}

func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	// convert entity to strongly typed collection
	collection, ok := entity.(*flow.Collection)
	if !ok {
		e.log.Error().Msgf("invalid entity type (%T)", entity)
	}

	err := e.handleCollection(originID, collection)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
		return
	}
}

// a block can't be executed if its collection is missing.
// since a collection can belong to multiple blocks, we need to
// find all the blocks that are needing this collection, and then
// check if any of these block becomes executable and execut it if
// is.
func (e *Engine) handleCollection(originID flow.Identifier, collection *flow.Collection) error {

	// TODO: bail if have seen this collection before.
	err := e.collections.Store(collection)
	if err != nil {
		return fmt.Errorf("cannot store collection: %w", err)
	}

	collID := collection.ID()

	return e.mempool.BlockByCollection.Run(
		func(backdata *stdmap.BlockByCollectionBackdata) error {
			blockByCollectionID, exists := backdata.ByID(collID)

			// if we don't find any block for this collection, then
			// means we don't need this collection any more.
			// or it was ejected from the mempool when it was full.
			// either way, we will return
			if !exists {
				return nil
			}

			for _, executableBlock := range blockByCollectionID.ExecutableBlocks {
				completeCollection, ok := executableBlock.CompleteCollections[collID]
				if !ok {
					return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
				}

				if completeCollection.IsCompleted() {
					// already received transactions for this collection
					continue
				}

				// update the transactions of the collection
				// Note: it's guaranteed the transactions are for this collection, because
				// the collection id matches with the CollectionID from the collection guarantee
				completeCollection.Transactions = collection.Transactions

				// check if the block becomes executable
				e.executeBlockIfComplete(executableBlock)
			}

			// since we've received this collection, remove it from the colle
			backdata.Rem(collID)

			return nil
		},
	)
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

func newQueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (*queue.Queue, bool) {
	q := queue.NewQueue(blockify)
	return q, queues.Add(q)
}

// enqueue inserts block into matching queue or creates a new one
func enqueue(blockify queue.Blockify, queues *stdmap.QueuesBackdata) (*queue.Queue, bool) {
	for _, queue := range queues.All() {
		if queue.TryAdd(blockify) {
			return queue, true
		}
	}
	return newQueue(blockify, queues)
}

// check if the block's collections have been received,
// if yes, add the collection to the executable block
// if no, fetch the collection.
// if a block has 3 collection, it would be 3 reqs to fetch them.
// mark the collection belongs to the block,
// mark the block contains this collection.
func (e *Engine) matchOrRequestCollections(
	executableBlock *entity.ExecutableBlock,
	collectionsBackdata *stdmap.BlockByCollectionBackdata,
) error {

	// make sure that the requests are dispatched immediately by the requester
	defer e.request.Force()

	for _, guarantee := range executableBlock.Block.Payload.Guarantees {
		coll := &entity.CompleteCollection{
			Guarantee: guarantee,
		}
		executableBlock.CompleteCollections[guarantee.ID()] = coll

		// check if we have requested this collection before.
		// blocksNeedingCollection stores all the blocks that contain this collection

		if blocksNeedingCollection, exists := collectionsBackdata.ByID(guarantee.ID()); exists {
			// if we've requested this collection, it means other block might also contain this collection.
			// in this case, add this block to the map so that when the collection is received,
			// we could update the executable block
			blocksNeedingCollection.ExecutableBlocks[executableBlock.ID()] = executableBlock

			// since the collection is still being requested, we don't have the transactions
			// yet, so exit
			continue
		}

		// if we are not requesting this collection, then there are two cases here:
		// 1) we have never seen this collection
		// 2) we have seen this collection from some other block

		// if we've requested this collection, we will store it in the storage,
		// so check the storage to see whether we've seen it.
		collection, err := e.collections.ByID(guarantee.CollectionID)

		if err == nil {
			// we found the collection, update the transactions
			coll.Transactions = collection.Transactions
			continue
		}

		// check if there was exception
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("error while querying for collection: %w", err)
		}

		// the storage doesn't have this collection, meaning this is our first time seeing this
		// collection guarantee, create an entry to store in collectionsBackdata in order to
		// update the executable blocks when the collection is received.
		blocksNeedingCollection := &entity.BlocksByCollection{
			CollectionID:     guarantee.ID(),
			ExecutableBlocks: map[flow.Identifier]*entity.ExecutableBlock{executableBlock.ID(): executableBlock},
		}

		added := collectionsBackdata.Add(blocksNeedingCollection)
		if !added {
			// sanity check, should not happen, unless mempool implementation has a bug
			return fmt.Errorf("collection already mapped to block")
		}

		e.log.Debug().
			Hex("block_id", logging.Entity(executableBlock)).
			Hex("collection_id", logging.ID(guarantee.ID())).
			Msg("requesting collection")

		// queue the collection to be requested from one of the guarantors
		e.request.EntityByID(guarantee.ID(), filter.HasNodeID(guarantee.SignerIDs...))
	}

	return nil
}

func (e *Engine) ExecuteScriptAtBlockID(ctx context.Context, script []byte, arguments [][]byte, blockID flow.Identifier) ([]byte, error) {

	stateCommit, err := e.execState.StateCommitmentByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}

	block, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	blockView := e.execState.NewView(stateCommit)

	return e.computationManager.ExecuteScript(script, arguments, block, blockView)
}

func (e *Engine) GetAccount(ctx context.Context, addr flow.Address, blockID flow.Identifier) (*flow.Account, error) {
	stateCommit, err := e.execState.StateCommitmentByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}

	block, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	blockView := e.execState.NewView(stateCommit)

	return e.computationManager.GetAccount(addr, block, blockView)
}

func (e *Engine) handleComputationResult(
	ctx context.Context,
	result *execution.ComputationResult,
	startState flow.StateCommitment,
) (flow.StateCommitment, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock)).
		Msg("received computation result")

	// There is one result per transaction
	e.metrics.ExecutionTotalExecutedTransactions(len(result.TransactionResult))

	receipt, err := e.saveExecutionResults(
		ctx,
		result.ExecutableBlock,
		result.StateSnapshots,
		result.Events,
		result.TransactionResult,
		startState,
	)

	if err != nil {
		return nil, err
	}

	err = e.providerEngine.BroadcastExecutionReceipt(ctx, receipt)
	if err != nil {
		return nil, fmt.Errorf("could not send broadcast order: %w", err)
	}

	return receipt.ExecutionResult.FinalStateCommit, nil
}

// save the execution result of a block
func (e *Engine) saveExecutionResults(
	ctx context.Context,
	executableBlock *entity.ExecutableBlock,
	stateInteractions []*delta.Snapshot,
	events []flow.Event,
	txResults []flow.TransactionResult,
	startState flow.StateCommitment,
) (*flow.ExecutionReceipt, error) {

	span, childCtx := e.tracer.StartSpanFromContext(ctx, trace.EXESaveExecutionResults)
	defer span.Finish()

	originalState := startState

	err := e.execState.PersistStateInteractions(childCtx, executableBlock.ID(), stateInteractions)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return nil, err
	}

	chunks := make([]*flow.Chunk, len(stateInteractions))

	// TODO: check current state root == startState
	var endState flow.StateCommitment = startState

	for i, view := range stateInteractions {
		// TODO: deltas should be applied to a particular state
		var err error
		endState, err = e.execState.CommitDelta(childCtx, view.Delta, startState)
		if err != nil {
			return nil, fmt.Errorf("failed to apply chunk delta: %w", err)
		}

		var collectionID flow.Identifier

		// account for system chunk being last
		if i < len(stateInteractions)-1 {
			collectionGuarantee := executableBlock.Block.Payload.Guarantees[i]
			completeCollection := executableBlock.CompleteCollections[collectionGuarantee.ID()]
			collectionID = completeCollection.Collection().ID()
		} else {
			collectionID = flow.ZeroID
		}

		chunk := generateChunk(i, startState, endState, collectionID)

		// chunkDataPack
		allRegisters := view.AllRegisters()

		values, proofs, err := e.execState.GetRegistersWithProofs(childCtx, chunk.StartState, allRegisters)
		if err != nil {
			return nil, fmt.Errorf(
				"error reading registers with proofs for chunk number [%v] of block [%x] ", i, executableBlock.ID(),
			)
		}

		chdp := generateChunkDataPack(chunk, collectionID, allRegisters, values, proofs)

		err = e.execState.PersistChunkDataPack(childCtx, chdp)
		if err != nil {
			return nil, fmt.Errorf("failed to save chunk data pack: %w", err)
		}

		// TODO use view.SpockSecret() as an input to spock generator
		chunks[i] = chunk
		startState = endState
	}

	err = e.execState.PersistStateCommitment(childCtx, executableBlock.ID(), endState)
	if err != nil {
		return nil, fmt.Errorf("failed to store state commitment: %w", err)
	}

	executionResult, err := e.generateExecutionResultForBlock(childCtx, executableBlock.Block, chunks, endState)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution result: %w", err)
	}

	receipt, err := e.generateExecutionReceipt(childCtx, executionResult, stateInteractions)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution receipt: %w", err)
	}

	// not update the highest executed until the result and receipts are saved.
	// TODO: better to save result, receipt and the latest height in one transaction
	err = e.execState.UpdateHighestExecutedBlockIfHigher(childCtx, executableBlock.Block.Header)
	if err != nil {
		return nil, fmt.Errorf("failed to update highest executed block: %w", err)
	}

	err = func() error {
		span, _ := e.tracer.StartSpanFromContext(childCtx, trace.EXESaveTransactionEvents)
		defer span.Finish()

		blockID := executableBlock.ID()
		if len(events) > 0 {
			err = e.events.Store(blockID, events)
			if err != nil {
				return fmt.Errorf("failed to store events: %w", err)
			}
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	err = func() error {
		span, _ := e.tracer.StartSpanFromContext(childCtx, trace.EXESaveTransactionResults)
		defer span.Finish()
		blockID := executableBlock.ID()
		err = e.transactionResults.BatchStore(blockID, txResults)
		if err != nil {
			return fmt.Errorf("failed to store transaction result error: %w", err)
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(executableBlock)).
		Hex("start_state", originalState).
		Hex("final_state", endState).
		Msg("saved computation results")

	return receipt, nil
}

// logExecutableBlock logs all data about an executable block
// over time we should skip this
func (e *Engine) logExecutableBlock(eb *entity.ExecutableBlock) {
	// log block
	e.log.Info().
		Hex("block_id", logging.Entity(eb)).
		Hex("prev_block_id", logging.ID(eb.Block.Header.ParentID)).
		Uint64("block_height", eb.Block.Header.Height).
		Int("number_of_collections", len(eb.Collections())).
		RawJSON("block_header", logging.AsJSON(eb.Block.Header)).
		Msg("extensive log: block header")

	// logs transactions
	for i, col := range eb.Collections() {
		for j, tx := range col.Transactions {
			e.log.Info().
				Hex("block_id", logging.Entity(eb)).
				Int("block_height", int(eb.Block.Header.Height)).
				Hex("prev_block_id", logging.ID(eb.Block.Header.ParentID)).
				Int("collection_index", i).
				Int("tx_index", j).
				Hex("collection_id", logging.ID(col.Guarantee.CollectionID)).
				Hex("tx_hash", logging.Entity(tx)).
				Hex("start_state_commitment", eb.StartState).
				RawJSON("transaction", logging.AsJSON(tx)).
				Msg("extensive log: executed tx content")
		}
	}
}

// generateChunk creates a chunk from the provided computation data.
func generateChunk(colIndex int, startState, endState flow.StateCommitment, colID flow.Identifier) *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(colIndex),
			StartState:      startState,
			// TODO: include real, event collection hash, currently using the collection ID to generate a different Chunk ID
			// Otherwise, the chances of there being chunks with the same ID before all these TODOs are done is large, since
			// startState stays the same if blocks are empty
			EventCollection: colID,
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
	ctx context.Context,
	block *flow.Block,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(ctx, block.Header.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get execution result ID for parent block (%v): %w",
			block.Header.ParentID, err)
	}

	er := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: previousErID,
			BlockID:          block.ID(),
			FinalStateCommit: endState,
			Chunks:           chunks,
		},
	}

	return er, nil
}

func (e *Engine) generateExecutionReceipt(
	ctx context.Context,
	result *flow.ExecutionResult,
	stateInteractions []*delta.Snapshot,
) (*flow.ExecutionReceipt, error) {

	spocks := make([]crypto.Signature, len(stateInteractions))

	for i, stateInteraction := range stateInteractions {
		spock, err := e.me.SignFunc(stateInteraction.SpockSecret, e.spockHasher, crypto.SPOCKProve)

		if err != nil {
			return nil, fmt.Errorf("error while generating SPoCK: %w", err)
		}
		spocks[i] = spock
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult:   *result,
		Spocks:            spocks,
		ExecutorSignature: crypto.Signature{},
		ExecutorID:        e.me.NodeID(),
	}

	// generates a signature over the execution result
	id := receipt.ID()
	sig, err := e.me.Sign(id[:], e.receiptHasher)
	if err != nil {
		return nil, fmt.Errorf("could not sign execution result: %w", err)
	}

	receipt.ExecutorSignature = sig

	err = e.execState.PersistExecutionReceipt(ctx, receipt)
	if err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
		return nil, fmt.Errorf("could not persist execution result: %w", err)
	}

	return receipt, nil
}

// check whether we need to trigger state sync
// queue - the queue contains all the unexecuted blocks along the same fork.
// we will check the state sync if the number of sealed and unexecuted blocks
// has passed a certain threshold.
// we will sync state only for sealed blocks, since that guarantees
// the consensus nodes have seen the result, and the statecommitment
// has been approved by the consensus nodes.
func (e *Engine) checkStateSyncStart(queue *queue.Queue) {
	if e.syncingState.Load() {
		// state sync is already triggered, it won't trigger again.
		return
	}

	// getting the blocks for determining whether to trigger.
	// the queue head has the lowest height, which is also the first unexecuted block
	firstUnexecuted := queue.Head.Item
	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		e.log.Fatal().Err(err).Msg("failed to query last sealed")
	}

	startHeight, endHeight := firstUnexecuted.Height(), lastSealed.Height

	// check whether we should trigger
	if !shouldTriggerStateSync(startHeight, endHeight, e.syncThreshold) {
		return
	}

	err = e.startStateSync(startHeight, endHeight)
	if err != nil {
		e.log.Error().
			Err(err).
			Uint64("from", startHeight).
			Uint64("to", endHeight).Msg("failed to start state sync")
	}
}

// if the state sync is on, check whether it can be turned off by checking
// whether the executed block has passed the target height.
func (e *Engine) checkStateSyncStop(executedBlock *entity.ExecutableBlock) {
	if !e.syncingState.Load() {
		// state sync was not started
		return
	}

	reachedSyncTarget := executedBlock.Block.Header.Height >= e.syncHeight

	if !reachedSyncTarget {
		// have not reached sync target
		return
	}

	// reached the sync target, we should turn off the syncing
	turnedOff := e.syncingState.CAS(true, false)
	if turnedOff {
		e.metrics.ExecutionSync(false)
	}
}

// check whether state sync should be triggered by taking
// the start and end heights for sealed and unexecuted blocks,
// as well as a threshold
func shouldTriggerStateSync(startHeight, endHeight uint64, threshold int) bool {
	return int64(endHeight)-int64(startHeight) > int64(threshold)
}

func (e *Engine) startStateSync(fromHeight, toHeight uint64) error {
	if !e.syncingState.CAS(false, true) {
		// some other process has already entered the startStateSync
		return nil
	}

	e.metrics.ExecutionSync(true)

	e.syncHeight = toHeight

	otherNodes, err := e.state.Final().Identities(
		filter.And(filter.HasRole(flow.RoleExecution), e.me.NotMeFilter(), e.syncFilter))

	if err != nil {
		return fmt.Errorf("error while finding other execution nodes identities")
	}

	if len(otherNodes) == 0 {
		e.log.Error().Msg("no available execution node to sync state from")
		e.syncingState.CAS(true, false)
		return nil
	}

	randomExecutionNode := otherNodes[rand.Intn(len(otherNodes))]

	exeStateReq := messages.ExecutionStateSyncRequest{
		FromHeight: fromHeight,
		ToHeight:   toHeight,
	}

	e.log.Debug().
		Hex("target_node", logging.Entity(randomExecutionNode)).
		Uint64("from", fromHeight).
		Uint64("to", toHeight).
		Msg("requesting execution state sync")

	// TODO: running periodically
	err = e.syncConduit.Submit(&exeStateReq, randomExecutionNode.NodeID)

	if err != nil {
		return fmt.Errorf("error while sending state sync req to other node (%v): %w",
			randomExecutionNode,
			err)
	}
	return nil
}

// handle the state sync request from other execution.
// the state sync requests are for sealed blocks.
// we will check if the requested heights have been sealed and
// executed, return return the state deltas as much as possible.
func (e *Engine) handleStateSyncRequest(
	originID flow.Identifier,
	req *messages.ExecutionStateSyncRequest) error {

	// the request must be from an execution node
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid role for requesting state synchronization: %v, %s", originID, id.Role)
	}

	// validate that from height must be smaller than to height
	if req.FromHeight >= req.ToHeight {
		return engine.NewInvalidInputErrorf("invalid state sync request (from: %x, to: %d)",
			req.FromHeight, req.ToHeight)
	}

	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get last sealed: %w", err)
	}

	sealedHeight := lastSealed.Height

	// ignore requests for unsealed height
	if req.FromHeight > sealedHeight {
		e.log.Info().
			Uint64("sealed", sealedHeight).
			Uint64("from", req.FromHeight).
			Uint64("to", req.ToHeight).
			Msg("requesting unsealed height")
		return nil
	}

	// fromHeight, toHeight must be sealed height
	fromHeight, toHeight := req.FromHeight, req.ToHeight
	if toHeight > sealedHeight {
		toHeight = sealedHeight
	}

	// for each height starting from fromHeight to toHeight,
	// query the statecommitment, and if exists, send the
	// state delta
	// TOOD: add context
	ctx := context.Background()
	err = e.deltaRange(ctx, fromHeight, toHeight,
		func(delta *messages.ExecutionStateDelta) {
			err := e.syncConduit.Submit(delta, originID)
			if err != nil {
				e.log.Error().Err(err).Msg("could not submit block delta")
			}
		})

	if err != nil {
		return fmt.Errorf("could not send deltas: %w", err)
	}

	return nil
}

func (e *Engine) deltaRange(ctx context.Context, fromHeight uint64, toHeight uint64,
	onDelta func(*messages.ExecutionStateDelta)) error {

	for height := fromHeight; height <= toHeight; height++ {
		header, err := e.state.AtHeight(height).Head()
		if err != nil {
			return fmt.Errorf("could not query block header at height: %v", height)
		}

		blockID := header.ID()
		_, err = e.execState.StateCommitmentByBlockID(ctx, blockID)

		if err == nil {
			// this block has been executed, we will send the delta
			delta, err := e.execState.RetrieveStateDelta(ctx, blockID)
			if err != nil {
				return fmt.Errorf("could not retrieve state delta for block %v, %w", blockID, err)
			}

			onDelta(delta)

		} else if !errors.Is(err, storage.ErrNotFound) {
			// this block has not been executed, continue
			continue
		} else {
			return fmt.Errorf("could not query statecommitment for height: %v", height)
		}
	}

	return nil
}

func (e *Engine) handleStateDeltaResponse(originID flow.Identifier, delta *messages.ExecutionStateDelta) error {

	// the request must be from an execution node
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid role for sending state deltas: %v, %s", originID, id.Role)
	}

	// check if the block has been executed already
	// delta ID is block ID
	blockID := delta.ID()
	_, err = e.execState.StateCommitmentByBlockID(context.Background(), blockID)

	if err == nil {
		// the block has been executed, ignore
		return nil
	}

	// exception
	if !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not get know block was executed or not: %w", err)
	}

	// block not executed yet, check if the block has been sealed
	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("failed to query last sealed height")
	}

	blockHeight := delta.ExecutableBlock.Block.Header.Height
	isUnsealed := blockHeight > lastSealed.Height

	if isUnsealed {
		// we never query delta for unsealed blocks, ignore
		return nil
	}

	err = e.validateStateDelta(delta)
	if err != nil {
		return fmt.Errorf("failed to validate the state delta: %w", err)
	}

	// TODO: validate the delta with the child block's statecommitment
	// TODO: execute the block if executable
	// TODO: add collection
	// TODO: acquire lock

	e.syncDeltas.Add(delta)

	return nil
}

func (e *Engine) validateStateDelta(delta *messages.ExecutionStateDelta) error {
	// must match the statecommitment for parent block
	parentCommitment, err := e.execState.StateCommitmentByBlockID(context.Background(), delta.ParentID())
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not get parent sttecommitment: %w", err)
	}

	// the parent block has not been executed yet, skip
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}

	if !bytes.Equal(parentCommitment, delta.StartState) {
		return engine.NewInvalidInputErrorf("internal inconsistency with delta for block (%v) - state commitment for parent retrieved from DB (%x) different from start state in delta! (%x)",
			delta.ParentID(),
			parentCommitment,
			delta.StartState)
	}

	return nil
}

func (e *Engine) applyStateDelta(delta *messages.ExecutionStateDelta) {
	// TODO - validate state sync, reject invalid messages
	executionReceipt, err := e.saveExecutionResults(
		context.Background(),
		&delta.ExecutableBlock,
		delta.StateInteractions,
		delta.Events,
		delta.TransactionResults,
		delta.StartState,
	)

	if err != nil {
		e.log.Fatal().Err(err).Msg("fatal error while processing sync message")
	}

	if !bytes.Equal(executionReceipt.ExecutionResult.FinalStateCommit, delta.EndState) {
		e.log.Fatal().
			Hex("saved_state", executionReceipt.ExecutionResult.FinalStateCommit).
			Hex("delta_end_state", delta.EndState).
			Hex("delta_start_state", delta.StartState).
			Err(err).Msg("processing sync message produced unexpected state commitment")
	}
}

// generateChunkDataPack creates a chunk data pack
func generateChunkDataPack(
	chunk *flow.Chunk,
	collectionID flow.Identifier,
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
		CollectionID:    collectionID,
	}
}
