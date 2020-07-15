package ingestion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

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
	"github.com/dapperlabs/flow-go/model/events"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/module/mempool/queue"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine receives and saves incoming blocks.
type Engine struct {
	unit                                *engine.Unit
	log                                 zerolog.Logger
	me                                  module.Local
	state                               protocol.State
	receiptHasher                       hash.Hasher // used as hasher to sign the execution receipt
	conduit                             network.Conduit
	collectionConduit                   network.Conduit
	syncConduit                         network.Conduit
	blocks                              storage.Blocks
	payloads                            storage.Payloads
	collections                         storage.Collections
	events                              storage.Events
	transactionResults                  storage.TransactionResults
	computationManager                  computation.ComputationManager
	providerEngine                      provider.ProviderEngine
	blockSync                           module.BlockRequester
	mempool                             *Mempool
	execState                           state.ExecutionState
	wg                                  sync.WaitGroup
	syncWg                              sync.WaitGroup
	syncModeThreshold                   uint64 // how many consecutive orphaned blocks trigger sync
	syncInProgress                      *atomic.Bool
	syncTargetBlockID                   atomic.Value
	stateSync                           executionSync.StateSynchronizer
	metrics                             module.ExecutionMetrics
	tracer                              module.Tracer
	extensiveLogging                    bool
	collectionRequestTimeout            time.Duration
	maximumCollectionRequestRetryNumber uint
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
	transactionResults storage.TransactionResults,
	executionEngine computation.ComputationManager,
	providerEngine provider.ProviderEngine,
	blockSync module.BlockRequester,
	execState state.ExecutionState,
	syncThreshold uint64,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	extLog bool,
	collectionRequestTimeout time.Duration,
	maximumCollectionRequestRetryNumber uint,
) (*Engine, error) {
	log := logger.With().Str("engine", "blocks").Logger()

	mempool := newMempool()

	eng := Engine{
		unit:                                engine.NewUnit(),
		log:                                 log,
		me:                                  me,
		state:                               state,
		receiptHasher:                       utils.NewExecutionReceiptHasher(),
		blocks:                              blocks,
		payloads:                            payloads,
		collections:                         collections,
		events:                              events,
		transactionResults:                  transactionResults,
		computationManager:                  executionEngine,
		providerEngine:                      providerEngine,
		blockSync:                           blockSync,
		mempool:                             mempool,
		execState:                           execState,
		syncModeThreshold:                   syncThreshold,
		syncInProgress:                      atomic.NewBool(false),
		stateSync:                           executionSync.NewStateSynchronizer(execState),
		metrics:                             metrics,
		tracer:                              tracer,
		extensiveLogging:                    extLog,
		collectionRequestTimeout:            collectionRequestTimeout,
		maximumCollectionRequestRetryNumber: maximumCollectionRequestRetryNumber,
	}

	con, err := net.Register(engine.BlockProvider, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register engine: %w", err)
	}

	collConduit, err := net.Register(engine.CollectionProvider, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register collection provider engine: %w", err)
	}

	syncConduit, err := net.Register(engine.ExecutionSync, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register execution blockSync engine: %w", err)
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
			engine.LogError(e.log, err)
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

		// stop all timers
		_ = e.mempool.BlockByCollection.Run(func(backdata *stdmap.BlockByCollectionBackdata) error {
			for _, ent := range backdata.All() {
				blocksByCollection, ok := ent.(*entity.BlocksByCollection)
				if !ok {
					panic(fmt.Sprintf("unknown type in BlockByCollection mempool: %T", ent))
				}
				if blocksByCollection.TimeoutTimer != nil {
					blocksByCollection.TimeoutTimer.Stop()
				}
			}
			return nil
		})

		e.Wait()
	})
}

func (e *Engine) Wait() {
	e.wg.Wait()     // wait for block execution
	e.syncWg.Wait() // wait for sync
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		log := e.log.With().Hex("origin", logging.ID(originID)).Logger()
		ctx := context.Background()

		var err error
		switch v := event.(type) {
		case *events.SyncedBlock:
			log.Debug().Hex("block_id", logging.Entity(v.Block.Header)).
				Uint64("block_view", v.Block.Header.View).
				Uint64("block_height", v.Block.Header.Height).
				Msg("received synced block")
			p := &messages.BlockProposal{
				Header: v.Block.Header, Payload: v.Block.Payload}
			err = e.handleBlockProposal(ctx, p)
		case *messages.BlockProposal:
			log.Debug().Hex("block_id", logging.Entity(v.Header)).
				Hex("parent_id", v.Header.ParentID[:]).
				Uint64("block_view", v.Header.View).
				Uint64("block_height", v.Header.Height).
				Hex("block_proposal", logging.Entity(v.Header)).Msg("received block proposal")
			err = e.handleBlockProposal(ctx, v)
		case *messages.CollectionResponse:
			log.Debug().Hex("collection_id", logging.Entity(v.Collection)).Msg("received collection response")
			err = e.handleCollectionResponse(ctx, v)
		case *messages.ExecutionStateDelta:
			log.Debug().
				Hex("block_id", logging.Entity(v.Block)).
				Uint64("block_height", v.Block.Header.Height).
				Msg("received block delta")
			err = e.handleExecutionStateDelta(ctx, v, originID)
		case *messages.ExecutionStateSyncRequest:
			log.Debug().Hex("current_block_id", logging.ID(v.CurrentBlockID)).
				Hex("target_block_id", logging.ID(v.TargetBlockID)).
				Msg("received execution state sync request")
			return e.onExecutionStateSyncRequest(ctx, originID, v)
		default:
			err = fmt.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return fmt.Errorf("could not process event (%T): %w", event, err)
		}
		return nil
	})
}

// Main handling

func (e *Engine) handleBlockProposal(ctx context.Context, proposal *messages.BlockProposal) error {

	block := &flow.Block{
		Header:  proposal.Header,
		Payload: proposal.Payload,
	}

	e.metrics.StartBlockReceivedToExecuted(block.ID())

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: make(map[flow.Identifier]*entity.CompleteCollection),
	}

	return e.mempool.Run(
		func(
			blockByCollection *stdmap.BlockByCollectionBackdata,
			executionQueues *stdmap.QueuesBackdata,
			orphanQueues *stdmap.QueuesBackdata,
		) error {
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
				if queue.Height() < e.syncModeThreshold {
					return nil
				}
				if e.syncInProgress.CAS(false, true) {
					// Start sync mode - initializing would require DB operation and will stop processing blocks here
					// which is exactly what we want
					e.StartSync(ctx, queue.Head.Item.(*entity.ExecutableBlock))
				}
				return nil
			}

			stateCommitment, err := e.execState.StateCommitmentByBlockID(ctx, block.Header.ParentID)
			// if state commitment doesn't exist and there are no known blocks which will produce
			// it soon (execution queue) then we save it as orphaned
			if errors.Is(err, storage.ErrNotFound) {
				queue, added := enqueue(executableBlock, orphanQueues)
				if !added {
					panic(fmt.Sprintf("could not enqueue orphaned block"))
				}
				e.tryRequeueOrphans(executableBlock, queue, orphanQueues)
				e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Hex("parent_id", logging.ID(executableBlock.Block.Header.ParentID)).Msg("added block to new orphan queue")
				// special case when sync threshold is reached
				if queue.Height() < e.syncModeThreshold {
					return nil
				}
				if e.syncInProgress.CAS(false, true) {
					// Start sync mode - initializing would require DB operation and will stop processing blocks here
					// which is exactly what we want
					e.StartSync(ctx, queue.Head.Item.(*entity.ExecutableBlock))
				}
				return nil
			}
			// any other error while accessing storage - panic
			if err != nil {
				panic(fmt.Sprintf("unexpected error while accessing storage, shutting down: %v", err))
			}

			//if block has state commitment, it has all parents blocks
			err = e.matchOrRequestCollections(executableBlock, blockByCollection)
			if err != nil {
				return fmt.Errorf("cannot send collection requests: %w", err)
			}

			executableBlock.StartState = stateCommitment
			newQueue, added := enqueue(executableBlock, executionQueues) // TODO - redundant? - should always produce new queue (otherwise it would be enqueued at the beginning)
			if !added {
				panic(fmt.Sprintf("could enqueue block for execution: %s", err))
			}
			e.log.Debug().Hex("block_id", logging.Entity(executableBlock.Block)).Msg("added block to execution queue")

			e.tryRequeueOrphans(executableBlock, newQueue, orphanQueues)

			// If the block was empty
			e.executeBlockIfComplete(executableBlock)

			return nil
		},
	)
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

func (e *Engine) executeBlock(ctx context.Context, executableBlock *entity.ExecutableBlock) {
	defer e.wg.Done()

	span, ctx := e.tracer.StartSpanFromContext(ctx, trace.EXEExecuteBlock)
	defer span.Finish()

	view := e.execState.NewView(executableBlock.StartState)
	e.log.Info().
		Hex("block_id", logging.Entity(executableBlock.Block)).
		Msg("executing block")

	computationResult, err := e.computationManager.ComputeBlock(ctx, executableBlock, view)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while computing block")
		return
	}

	e.metrics.FinishBlockReceivedToExecuted(executableBlock.Block.ID())
	e.metrics.ExecutionGasUsedPerBlock(computationResult.GasUsed)
	e.metrics.ExecutionStateReadsPerBlock(computationResult.StateReads)

	finalState, err := e.handleComputationResult(ctx, computationResult, executableBlock.StartState)
	if err != nil {
		e.log.Err(err).
			Hex("block_id", logging.Entity(executableBlock.Block)).
			Msg("error while handing computation results")
		return
	}

	diskTotal, err := e.execState.DiskSize()
	if err != nil {
		e.log.Err(err).Msg("could not get execution state disk size")
	}

	e.metrics.ExecutionStateStorageDiskTotal(diskTotal)
	e.metrics.ExecutionStorageStateCommitment(int64(len(finalState)))

	err = e.mempool.Run(
		func(
			blockByCollection *stdmap.BlockByCollectionBackdata,
			executionQueues *stdmap.QueuesBackdata,
			_ *stdmap.QueuesBackdata,
		) error {
			executionQueue, exists := executionQueues.ByID(executableBlock.Block.ID())
			if !exists {
				return fmt.Errorf("fatal error - executed block not present in execution queue")
			}

			_, newQueues := executionQueue.Dismount()

			for _, queue := range newQueues {
				newExecutableBlock := queue.Head.Item.(*entity.ExecutableBlock)
				newExecutableBlock.StartState = finalState

				err := e.matchOrRequestCollections(newExecutableBlock, blockByCollection)
				if err != nil {
					return fmt.Errorf("cannot send collection requests: %w", err)
				}

				added := executionQueues.Add(queue)
				if !added {
					return fmt.Errorf("fatal error - child block already in execution queue")
				}

				e.executeBlockIfComplete(newExecutableBlock)
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
		Msg("block executed")
	e.metrics.ExecutionLastExecutedBlockView(executableBlock.Block.Header.View)
}

func (e *Engine) executeBlockIfComplete(eb *entity.ExecutableBlock) bool {
	if eb.IsComplete() {
		e.log.Debug().
			Hex("block_id", logging.Entity(eb.Block)).
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

func (e *Engine) handleCollectionResponse(ctx context.Context, response *messages.CollectionResponse) error {

	collection := response.Collection
	collID := collection.ID()

	err := e.collections.Store(&collection)
	if err != nil {
		return fmt.Errorf("cannot store collection: %w", err)
	}

	return e.mempool.BlockByCollection.Run(
		func(backdata *stdmap.BlockByCollectionBackdata) error {
			blockByCollectionId, exists := backdata.ByID(collID)
			if !exists {
				return fmt.Errorf("could not find block for collection")
			}

			blockByCollectionId.TimeoutTimer.Stop()
			//set to nil to prevent stopping twice while shutting down
			blockByCollectionId.TimeoutTimer = nil

			executableBlocks := blockByCollectionId.ExecutableBlocks

			for _, executableBlock := range executableBlocks {

				completeCollection, ok := executableBlock.CompleteCollections[collID]
				if !ok {
					return fmt.Errorf("cannot handle collection: internal inconsistency - collection pointing to block which does not contain said collection")
				}
				// already received transactions for this collection
				// TODO - check if data stored is the same
				if completeCollection.Transactions != nil {
					continue
				}

				completeCollection.Transactions = collection.Transactions

				e.executeBlockIfComplete(executableBlock)
			}
			backdata.Rem(collID)

			return nil
		},
	)
}

func (e *Engine) findCollectionNodesForGuarantee(
	blockID flow.Identifier,
	guarantee *flow.CollectionGuarantee,
) ([]flow.Identifier, error) {

	filter := filter.And(
		filter.HasRole(flow.RoleCollection),
		filter.HasNodeID(guarantee.SignerIDs...))

	identities, err := e.state.AtBlockID(blockID).Identities(filter)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve identities (%s): %w", blockID, err)
	}
	if len(identities) < 1 {
		return nil, fmt.Errorf("no collection identity found")
	}

	identifiers := make([]flow.Identifier, len(identities))
	for i, id := range identities {
		identifiers[i] = id.NodeID
	}

	return identifiers, nil
}

func (e *Engine) onExecutionStateSyncRequest(
	ctx context.Context,
	originID flow.Identifier,
	req *messages.ExecutionStateSyncRequest,
) error {
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid role for requesting state synchronization: %s", id.Role)
	}

	err = e.stateSync.DeltaRange(
		ctx,
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

func (e *Engine) submitCollectionRequest(timer **time.Timer, collID flow.Identifier, recipients []flow.Identifier, retry uint) {

	request := &messages.CollectionRequest{
		ID:    collID,
		Nonce: rand.Uint64(),
	}

	if retry >= e.maximumCollectionRequestRetryNumber {
		e.log.Error().Hex("collection_id", collID[:]).Msg("exceeded maximum number of retries of collection request")
		return
	}

	if retry > 0 {
		e.log.Info().Hex("collection_id", collID[:]).Uint("retry", retry).Msg("retrying request for collection")
		e.metrics.ExecutionCollectionRequestRetried()
	} else {
		e.metrics.ExecutionCollectionRequestSent()
	}

	err := e.collectionConduit.Submit(
		request,
		recipients...,
	)
	if err != nil {
		e.log.Warn().Err(err).Hex("collection_id", collID[:]).Msg("cannot submit collection requests")
	}

	*timer = time.AfterFunc(e.collectionRequestTimeout, func() {
		e.submitCollectionRequest(timer, collID, recipients, retry+1)
	})
}

func (e *Engine) matchOrRequestCollections(
	executableBlock *entity.ExecutableBlock,
	backdata *stdmap.BlockByCollectionBackdata,
) error {

	for _, guarantee := range executableBlock.Block.Payload.Guarantees {
		var transactions []*flow.TransactionBody
		maybeBlockByCollection, exists := backdata.ByID(guarantee.ID())

		if exists {
			if _, exists := maybeBlockByCollection.ExecutableBlocks[executableBlock.ID()]; exists {
				e.log.Info().Hex("block_id", logging.Entity(executableBlock)).Msg("requesting collections for block with already pending requests. Ignoring requests.")
			}
			maybeBlockByCollection.ExecutableBlocks[executableBlock.ID()] = executableBlock
		} else {
			collection, err := e.collections.ByID(guarantee.CollectionID)

			if err == nil {
				transactions = collection.Transactions
			} else if errors.Is(err, storage.ErrNotFound) {
				maybeBlockByCollection = &entity.BlocksByCollection{
					CollectionID:     guarantee.ID(),
					ExecutableBlocks: map[flow.Identifier]*entity.ExecutableBlock{executableBlock.ID(): executableBlock},
				}

				added := backdata.Add(maybeBlockByCollection)
				if !added {
					return fmt.Errorf("collection already mapped to block")
				}

				collectionGuaranteesIdentifiers, err := e.findCollectionNodesForGuarantee(
					executableBlock.Block.ID(),
					guarantee,
				)
				if err != nil {
					return err
				}

				e.log.Debug().
					Hex("block_id", logging.Entity(executableBlock.Block)).
					Hex("collection_id", logging.ID(guarantee.ID())).
					Msg("requesting collection")

				e.submitCollectionRequest(&maybeBlockByCollection.TimeoutTimer, guarantee.ID(), collectionGuaranteesIdentifiers, 0)
			} else {
				return fmt.Errorf("error while querying for collection: %w", err)
			}
		}

		executableBlock.CompleteCollections[guarantee.ID()] = &entity.CompleteCollection{
			Guarantee:    guarantee,
			Transactions: transactions,
		}
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
		Hex("block_id", logging.ID(result.ExecutableBlock.Block.ID())).
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

	err := e.execState.PersistStateInteractions(childCtx, executableBlock.Block.ID(), stateInteractions)
	if err != nil {
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

		chunk := generateChunk(i, startState, endState)

		// chunkDataPack
		allRegisters := view.AllRegisters()

		values, proofs, err := e.execState.GetRegistersWithProofs(childCtx, chunk.StartState, allRegisters)
		if err != nil {
			return nil, fmt.Errorf(
				"error reading registers with proofs for chunk number [%v] of block [%x] ", i, executableBlock.ID(),
			)
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

	err = e.execState.UpdateHighestExecutedBlockIfHigher(childCtx, executableBlock.Block.Header)
	if err != nil {
		return nil, fmt.Errorf("failed to update highest executed block: %w", err)
	}

	executionResult, err := e.generateExecutionResultForBlock(childCtx, executableBlock.Block, chunks, endState)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution result: %w", err)
	}

	receipt, err := e.generateExecutionReceipt(childCtx, executionResult)
	if err != nil {
		return nil, fmt.Errorf("could not generate execution receipt: %w", err)
	}

	err = func() error {
		span, _ := e.tracer.StartSpanFromContext(childCtx, trace.EXESaveTransactionResults)
		defer span.Finish()

		if len(events) > 0 {
			err = e.events.Store(executableBlock.ID(), events)
			if err != nil {
				return fmt.Errorf("failed to store events: %w", err)
			}
		}

		for _, te := range txResults {
			err = e.transactionResults.Store(executableBlock.ID(), &te)
			if err != nil {
				return fmt.Errorf("failed to store transaction error: %w", err)
			}
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
		Hex("block_id", logging.Entity(eb.Block)).
		Hex("prev_block_id", logging.ID(eb.Block.Header.ParentID)).
		Uint64("block_height", eb.Block.Header.Height).
		Int("number_of_collections", len(eb.Collections())).
		RawJSON("block_header", logging.AsJSON(eb.Block.Header)).
		Msg("extensive log: block header")

	// logs transactions
	for i, col := range eb.Collections() {
		for j, tx := range col.Transactions {
			e.log.Info().
				Hex("block_id", logging.Entity(eb.Block)).
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
	ctx context.Context,
	block *flow.Block,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(ctx, block.Header.ParentID)
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

	err = e.execState.PersistExecutionResult(ctx, block.ID(), *er)
	if err != nil {
		return nil, fmt.Errorf("could not persist execution result: %w", err)
	}

	return er, nil
}

func (e *Engine) generateExecutionReceipt(
	ctx context.Context,
	result *flow.ExecutionResult,
) (*flow.ExecutionReceipt, error) {

	receipt := &flow.ExecutionReceipt{
		ExecutionResult:   *result,
		Spocks:            nil, // TODO: include SPoCKs
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

	return receipt, nil
}

func (e *Engine) StartSync(ctx context.Context, firstKnown *entity.ExecutableBlock) {
	// find latest finalized block with state commitment
	// this way we maximise chance of path existing between it and fresh one
	// TODO - this doesn't make sense if we treat every block as finalized (MVP)

	targetBlockID := firstKnown.Block.Header.ParentID
	targetHeight := firstKnown.Block.Header.Height - 1

	e.syncTargetBlockID.Store(targetBlockID)

	e.log.Info().
		Hex("target_id", targetBlockID[:]).
		Uint64("target_height", targetHeight).
		Msg("starting state synchronization")

	lastExecutedHeight, lastExecutedBlockID, err := e.execState.GetHighestExecutedBlockID(ctx)
	if err != nil {
		e.log.Fatal().Err(err).Msg("error while starting sync - cannot find highest executed block")
	}

	if lastExecutedHeight == targetHeight && lastExecutedBlockID != targetBlockID {
		e.log.Error().Err(err).Msg("error while starting sync - first known not on same branch as last executed block")
		// Mark sync as no longer in progress, and allow any additional incoming blocks to kick off sync again with a different block
		e.syncInProgress.Store(false)
		return
	}

	e.log.Debug().
		Msgf("syncing from height %d to height %d", lastExecutedHeight, targetHeight)

	otherNodes, err := e.state.Final().Identities(filter.And(filter.HasRole(flow.RoleExecution), e.me.NotMeFilter()))
	if err != nil {
		e.log.Fatal().Err(err).Msg("error while finding other execution nodes identities")
		return
	}

	if len(otherNodes) < 1 {
		e.log.Debug().
			Msgf("no other execution nodes found, request last block instead at height %d", targetHeight)
		e.blockSync.RequestBlock(targetBlockID)
		return
	}

	// select other node at random
	// TODO - protocol which surveys other nodes for state
	// TODO - ability to sync from multiple servers
	// TODO - handle byzantine other node that does not send response
	otherNodeIdentity := otherNodes[rand.Intn(len(otherNodes))]

	exeStateReq := messages.ExecutionStateSyncRequest{
		CurrentBlockID: lastExecutedBlockID,
		TargetBlockID:  targetBlockID,
	}

	e.log.Debug().
		Hex("target_node", logging.Entity(otherNodeIdentity)).
		Hex("current_block_id", logging.ID(exeStateReq.CurrentBlockID)).
		Hex("target_block_id", logging.ID(exeStateReq.TargetBlockID)).
		Msg("requesting execution state sync")

	err = e.syncConduit.Submit(&exeStateReq, otherNodeIdentity.NodeID)

	if err != nil {
		e.log.Fatal().
			Err(err).
			Str("target_node_id", otherNodeIdentity.NodeID.String()).
			Msg("error while requesting state sync from other node")
	}
}

func (e *Engine) handleExecutionStateDelta(
	ctx context.Context,
	executionStateDelta *messages.ExecutionStateDelta,
	originID flow.Identifier,
) error {

	return e.mempool.SyncQueues.Run(func(backdata *stdmap.QueuesBackdata) error {
		log := e.log.With().
			Hex("block_id", logging.Entity(executionStateDelta.Block)).
			Uint64("block_height", executionStateDelta.Block.Header.Height).
			Logger()

		// try enqueue
		if queue, added := tryEnqueue(executionStateDelta, backdata); added {
			log.Debug().
				Msg("added block to existing orphan queue")

			e.tryRequeueOrphans(executionStateDelta, queue, backdata)
			return nil
		}

		stateCommitment, getStateCommitmentErr := e.execState.StateCommitmentByBlockID(ctx, executionStateDelta.ParentID())
		if getStateCommitmentErr != nil && !errors.Is(getStateCommitmentErr, storage.ErrNotFound) {
			log.Fatal().Msgf("unexpected error while accessing storage for sync deltas, shutting down: %v", getStateCommitmentErr)
		}

		newQueue, added := enqueue(executionStateDelta, backdata)
		if !added {
			log.Fatal().Msgf("cannot enqueue sync delta: %s", getStateCommitmentErr)
		}

		e.tryRequeueOrphans(executionStateDelta, newQueue, backdata)

		if errors.Is(getStateCommitmentErr, storage.ErrNotFound) {
			// if state commitment doesn't exist and there are no known deltas which will produce
			// it soon (sync queue) then we save it as orphaned
			return nil
		}

		if !bytes.Equal(stateCommitment, executionStateDelta.StartState) {
			return fmt.Errorf("internal inconsistency with delta - state commitment for parent retrieved from DB different from start state in delta! ")
		}

		e.syncWg.Add(1)
		go e.saveDelta(ctx, executionStateDelta)

		return nil
	})
}

func (e *Engine) saveDelta(ctx context.Context, executionStateDelta *messages.ExecutionStateDelta) {

	defer e.syncWg.Done()

	log := e.log.With().
		Hex("block_id", logging.Entity(executionStateDelta.Block)).
		Uint64("block_height", executionStateDelta.Block.Header.Height).
		Logger()

	// synchronize DB writing to avoid tx conflicts with multiple blocks arriving fast
	err := e.blocks.Store(executionStateDelta.Block)
	if err != nil {
		// It's possible for the parent of the target block to have arrived already. Don't fail here
		if !errors.Is(err, storage.ErrAlreadyExists) {
			log.Fatal().
				Err(err).Msg("could not store block from delta")
		}
	}

	for _, collection := range executionStateDelta.CompleteCollections {
		collection := collection.Collection()
		err := e.collections.Store(&collection)
		if err != nil {
			log.Fatal().
				Err(err).Msg("could not store collection from delta")
		}
	}

	// TODO - validate state sync, reject invalid messages, change provider
	executionReceipt, err := e.saveExecutionResults(
		ctx,
		&executionStateDelta.ExecutableBlock,
		executionStateDelta.StateInteractions,
		executionStateDelta.Events,
		executionStateDelta.TransactionResults,
		executionStateDelta.StartState,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("fatal error while processing sync message")
	}

	if !bytes.Equal(executionReceipt.ExecutionResult.FinalStateCommit, executionStateDelta.EndState) {
		log.Fatal().
			Hex("saved_state", executionReceipt.ExecutionResult.FinalStateCommit).
			Hex("delta_end_state", executionStateDelta.EndState).
			Hex("delta_start_state", executionStateDelta.StartState).
			Err(err).Msg("processing sync message produced unexpected state commitment")
	}

	targetBlockID := e.syncTargetBlockID.Load().(flow.Identifier)

	// last block was saved
	if targetBlockID == executionStateDelta.Block.ID() {
		log.Debug().Msg("final target sync block received, processing")

		err = e.mempool.Run(
			func(
				blockByCollection *stdmap.BlockByCollectionBackdata,
				executionQueues *stdmap.QueuesBackdata,
				orphanQueues *stdmap.QueuesBackdata,
			) error {
				var syncedQueue *queue.Queue
				hadQueue := false

				for _, q := range orphanQueues.All() {
					if q.Head.Item.(*entity.ExecutableBlock).Block.Header.ParentID == targetBlockID {
						syncedQueue = q
						hadQueue = true
						break
					}
				}
				if !hadQueue {
					log.Fatal().Msgf("orphan queues do not contain final block ID (%s)", targetBlockID)
				}

				orphanQueues.Rem(syncedQueue.ID())

				// if the state we generated from applying this is not equal to EndState we would have panicked earlier
				executableBlock := syncedQueue.Head.Item.(*entity.ExecutableBlock)
				executableBlock.StartState = executionStateDelta.EndState

				err = e.matchOrRequestCollections(executableBlock, blockByCollection)
				if err != nil {
					return fmt.Errorf("cannot send collection requests: %w", err)
				}

				if executableBlock.IsComplete() {
					added := executionQueues.Add(syncedQueue)
					if !added {
						log.Fatal().Msgf("cannot add queue to execution queues")
					}

					log.Debug().Msg("block complete - executing")

					e.wg.Add(1)
					go e.executeBlock(context.Background(), executableBlock)
				}
				log.Debug().Msg("final target sync block processed")

				return nil
			})

		if err != nil {
			log.Err(err).Msg("error while processing final target sync block")
		}

		return
	}

	err = e.mempool.SyncQueues.Run(func(backdata *stdmap.QueuesBackdata) error {

		executionQueue, exists := backdata.ByID(executionStateDelta.Block.ID())
		if !exists {
			return fmt.Errorf("fatal error - synced delta not present in sync queue")
		}
		_, newQueues := executionQueue.Dismount()
		for _, queue := range newQueues {
			if !bytes.Equal(
				queue.Head.Item.(*messages.ExecutionStateDelta).StartState,
				executionReceipt.ExecutionResult.FinalStateCommit,
			) {
				return fmt.Errorf("internal incosistency with delta - state commitment for after applying delta different from start state of next one! ")
			}

			added := backdata.Add(queue)
			if !added {
				return fmt.Errorf("fatal error cannot add children block to sync queue")
			}

			e.syncWg.Add(1)
			go e.saveDelta(ctx, queue.Head.Item.(*messages.ExecutionStateDelta))
		}
		backdata.Rem(executionStateDelta.Block.ID())
		return nil
	})

	if err != nil {
		log.Err(err).Msg("error while requeueing delta after saving")
	}

	log.Debug().Msg("finished processing sync delta")
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
