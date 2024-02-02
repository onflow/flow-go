package state

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

var ErrExecutionStatePruned = fmt.Errorf("execution state is pruned")
var ErrNotExecuted = fmt.Errorf("block not executed")

// ReadOnlyExecutionState allows to read the execution state
type ReadOnlyExecutionState interface {
	ScriptExecutionState

	// ChunkDataPackByChunkID retrieve a chunk data pack given the chunk ID.
	ChunkDataPackByChunkID(flow.Identifier) (*flow.ChunkDataPack, error)

	GetExecutionResultID(context.Context, flow.Identifier) (flow.Identifier, error)

	GetHighestExecutedBlockID(context.Context) (uint64, flow.Identifier, error)
}

// ScriptExecutionState is a subset of the `state.ExecutionState` interface purposed to only access the state
// used for script execution and not mutate the execution state of the blockchain.
type ScriptExecutionState interface {
	// NewStorageSnapshot creates a new ready-only view at the given block.
	NewStorageSnapshot(commit flow.StateCommitment, blockID flow.Identifier, height uint64) snapshot.StorageSnapshot

	// CreateStorageSnapshot creates a new ready-only view at the given block.
	// It returns:
	// - (nil, nil, storage.ErrNotFound) if block is unknown
	// - (nil, nil, state.ErrNotExecuted) if block is not executed
	// - (nil, nil, state.ErrExecutionStatePruned) if the execution state has been pruned
	CreateStorageSnapshot(blockID flow.Identifier) (snapshot.StorageSnapshot, *flow.Header, error)

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(flow.Identifier) (flow.StateCommitment, error)

	// Any error returned is exception
	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
}

func IsParentExecuted(state ReadOnlyExecutionState, header *flow.Header) (bool, error) {
	// sanity check, caller should not pass a root block
	if header.Height == 0 {
		return false, fmt.Errorf("root block does not have parent block")
	}
	return state.IsBlockExecuted(header.Height-1, header.ParentID)
}

// FinalizedExecutionState is an interface used to access the finalized execution state
type FinalizedExecutionState interface {
	GetHighestFinalizedExecuted() uint64
}

// TODO Many operations here are should be transactional, so we need to refactor this
// to store a reference to DB and compose operations and procedures rather then
// just being amalgamate of proxies for single transactions operation

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	ReadOnlyExecutionState

	UpdateHighestExecutedBlockIfHigher(context.Context, *flow.Header) error

	SaveExecutionResults(
		ctx context.Context,
		result *execution.ComputationResult,
	) error

	// only available with storehouse enabled
	// panic when called with storehouse disabled (which should be a bug)
	GetHighestFinalizedExecuted() uint64
}

type state struct {
	tracer             module.Tracer
	ls                 ledger.Ledger
	commits            storage.Commits
	blocks             storage.Blocks
	headers            storage.Headers
	collections        storage.Collections
	chunkDataPacks     storage.ChunkDataPacks
	results            storage.ExecutionResults
	myReceipts         storage.MyExecutionReceipts
	events             storage.Events
	serviceEvents      storage.ServiceEvents
	transactionResults storage.TransactionResults
	db                 *badger.DB

	registerStore execution.RegisterStore
	// when it is true, registers are stored in both register store and ledger
	// and register queries will send to the register store instead of ledger
	enableRegisterStore bool
}

// NewExecutionState returns a new execution state access layer for the given ledger storage.
func NewExecutionState(
	ls ledger.Ledger,
	commits storage.Commits,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	myReceipts storage.MyExecutionReceipts,
	events storage.Events,
	serviceEvents storage.ServiceEvents,
	transactionResults storage.TransactionResults,
	db *badger.DB,
	tracer module.Tracer,
	registerStore execution.RegisterStore,
	enableRegisterStore bool,
) ExecutionState {
	return &state{
		tracer:              tracer,
		ls:                  ls,
		commits:             commits,
		blocks:              blocks,
		headers:             headers,
		collections:         collections,
		chunkDataPacks:      chunkDataPacks,
		results:             results,
		myReceipts:          myReceipts,
		events:              events,
		serviceEvents:       serviceEvents,
		transactionResults:  transactionResults,
		db:                  db,
		registerStore:       registerStore,
		enableRegisterStore: enableRegisterStore,
	}

}

func makeSingleValueQuery(commitment flow.StateCommitment, id flow.RegisterID) (*ledger.QuerySingleValue, error) {
	return ledger.NewQuerySingleValue(ledger.State(commitment),
		convert.RegisterIDToLedgerKey(id),
	)
}

func RegisterEntriesToKeysValues(
	entries flow.RegisterEntries,
) (
	[]ledger.Key,
	[]ledger.Value,
) {
	keys := make([]ledger.Key, len(entries))
	values := make([]ledger.Value, len(entries))
	for i, entry := range entries {
		keys[i] = convert.RegisterIDToLedgerKey(entry.Key)
		values[i] = entry.Value
	}
	return keys, values
}

type LedgerStorageSnapshot struct {
	ledger     ledger.Ledger
	commitment flow.StateCommitment

	mutex     sync.RWMutex
	readCache map[flow.RegisterID]flow.RegisterValue // Guarded by mutex.
}

func NewLedgerStorageSnapshot(
	ldg ledger.Ledger,
	commitment flow.StateCommitment,
) snapshot.StorageSnapshot {
	return &LedgerStorageSnapshot{
		ledger:     ldg,
		commitment: commitment,
		readCache:  make(map[flow.RegisterID]flow.RegisterValue),
	}
}

func (storage *LedgerStorageSnapshot) getFromCache(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	bool,
) {
	storage.mutex.RLock()
	defer storage.mutex.RUnlock()

	value, ok := storage.readCache[id]
	return value, ok
}

func (storage *LedgerStorageSnapshot) getFromLedger(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	query, err := makeSingleValueQuery(storage.commitment, id)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	value, err := storage.ledger.GetSingleValue(query)
	if err != nil {
		return nil, fmt.Errorf(
			"error getting register (%s) value at %x: %w",
			id,
			storage.commitment,
			err)
	}

	return value, nil
}

func (storage *LedgerStorageSnapshot) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	value, ok := storage.getFromCache(id)
	if ok {
		return value, nil
	}

	value, err := storage.getFromLedger(id)
	if err != nil {
		return nil, err
	}

	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	storage.readCache[id] = value
	return value, nil
}

func (s *state) NewStorageSnapshot(
	commitment flow.StateCommitment,
	blockID flow.Identifier,
	height uint64,
) snapshot.StorageSnapshot {
	if s.enableRegisterStore {
		return storehouse.NewBlockEndStateSnapshot(s.registerStore, blockID, height)
	}
	return NewLedgerStorageSnapshot(s.ls, commitment)
}

func (s *state) CreateStorageSnapshot(
	blockID flow.Identifier,
) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := s.headers.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get header by block ID: %w", err)
	}

	// make sure the block is executed
	commit, err := s.commits.ByBlockID(blockID)
	if err != nil {
		// statecommitment not exists means the block hasn't been executed yet
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil, fmt.Errorf("block %v is never executed: %w", blockID, ErrNotExecuted)
		}

		return nil, header, fmt.Errorf("cannot get commit by block ID: %w", err)
	}

	// make sure we have trie state for this block
	ledgerHasState := s.ls.HasState(ledger.State(commit))
	if !ledgerHasState {
		return nil, header, fmt.Errorf("state not found in ledger for commit %x (block %v): %w", commit, blockID, ErrExecutionStatePruned)
	}

	if s.enableRegisterStore {
		isExecuted, err := s.registerStore.IsBlockExecuted(header.Height, blockID)
		if err != nil {
			return nil, header, fmt.Errorf("cannot check if block %v is executed: %w", blockID, err)
		}
		if !isExecuted {
			return nil, header, fmt.Errorf("block %v is not executed yet: %w", blockID, ErrNotExecuted)
		}
	}

	return s.NewStorageSnapshot(commit, blockID, header.Height), header, nil
}

type RegisterUpdatesHolder interface {
	UpdatedRegisters() flow.RegisterEntries
	UpdatedRegisterSet() map[flow.RegisterID]flow.RegisterValue
}

// CommitDelta takes a base storage snapshot and creates a new storage snapshot
// with the register updates from the given RegisterUpdatesHolder
// a new statecommitment is returned from the ledger, along with the trie update
// any error returned are exceptions
func CommitDelta(
	ldg ledger.Ledger,
	ruh RegisterUpdatesHolder,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (flow.StateCommitment, *ledger.TrieUpdate, execution.ExtendableStorageSnapshot, error) {

	updatedRegisters := ruh.UpdatedRegisters()
	keys, values := RegisterEntriesToKeysValues(updatedRegisters)
	baseState := baseStorageSnapshot.Commitment()
	update, err := ledger.NewUpdate(ledger.State(baseState), keys, values)

	if err != nil {
		return flow.DummyStateCommitment, nil, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	newState, trieUpdate, err := ldg.Set(update)
	if err != nil {
		return flow.DummyStateCommitment, nil, nil, fmt.Errorf("could not update ledger: %w", err)
	}

	newCommit := flow.StateCommitment(newState)

	newStorageSnapshot := baseStorageSnapshot.Extend(newCommit, ruh.UpdatedRegisterSet())

	return newCommit, trieUpdate, newStorageSnapshot, nil
}

func (s *state) StateCommitmentByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return s.commits.ByBlockID(blockID)
}

func (s *state) ChunkDataPackByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	chunkDataPack, err := s.chunkDataPacks.ByChunkID(chunkID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve stored chunk data pack: %w", err)
	}

	return chunkDataPack, nil
}

func (s *state) GetExecutionResultID(ctx context.Context, blockID flow.Identifier) (flow.Identifier, error) {
	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetExecutionResultID)
	defer span.End()

	result, err := s.results.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, err
	}
	return result.ID(), nil
}

func (s *state) SaveExecutionResults(
	ctx context.Context,
	result *execution.ComputationResult,
) error {
	span, childCtx := s.tracer.StartSpanFromContext(
		ctx,
		trace.EXEStateSaveExecutionResults)
	defer span.End()

	err := s.saveExecutionResults(ctx, result)
	if err != nil {
		return fmt.Errorf("could not save execution results: %w", err)
	}

	if s.enableRegisterStore {
		// save registers to register store
		err = s.registerStore.SaveRegisters(
			result.BlockExecutionResult.ExecutableBlock.Block.Header,
			result.BlockExecutionResult.AllUpdatedRegisters(),
		)

		if err != nil {
			return fmt.Errorf("could not save updated registers: %w", err)
		}
	}

	//outside batch because it requires read access
	err = s.UpdateHighestExecutedBlockIfHigher(childCtx, result.ExecutableBlock.Block.Header)
	if err != nil {
		return fmt.Errorf("cannot update highest executed block: %w", err)
	}
	return nil
}

func (s *state) saveExecutionResults(
	ctx context.Context,
	result *execution.ComputationResult,
) (err error) {
	header := result.ExecutableBlock.Block.Header
	blockID := header.ID()

	err = s.chunkDataPacks.Store(result.AllChunkDataPacks())
	if err != nil {
		return fmt.Errorf("can not store multiple chunk data pack: %w", err)
	}

	// Write Batch is BadgerDB feature designed for handling lots of writes
	// in efficient and atomic manner, hence pushing all the updates we can
	// as tightly as possible to let Badger manage it.
	// Note, that it does not guarantee atomicity as transactions has size limit,
	// but it's the closest thing to atomicity we could have
	batch := badgerstorage.NewBatch(s.db)

	defer func() {
		// Rollback if an error occurs during batch operations
		if err != nil {
			chunks := result.AllChunkDataPacks()
			chunkIDs := make([]flow.Identifier, 0, len(chunks))
			for _, chunk := range chunks {
				chunkIDs = append(chunkIDs, chunk.ID())
			}
			_ = s.chunkDataPacks.Remove(chunkIDs)
		}
	}()

	err = s.events.BatchStore(blockID, []flow.EventsList{result.AllEvents()}, batch)
	if err != nil {
		return fmt.Errorf("cannot store events: %w", err)
	}

	err = s.serviceEvents.BatchStore(blockID, result.AllServiceEvents(), batch)
	if err != nil {
		return fmt.Errorf("cannot store service events: %w", err)
	}

	err = s.transactionResults.BatchStore(
		blockID,
		result.AllTransactionResults(),
		batch)
	if err != nil {
		return fmt.Errorf("cannot store transaction result: %w", err)
	}

	executionResult := &result.ExecutionReceipt.ExecutionResult
	err = s.results.BatchStore(executionResult, batch)
	if err != nil {
		return fmt.Errorf("cannot store execution result: %w", err)
	}

	err = s.results.BatchIndex(blockID, executionResult.ID(), batch)
	if err != nil {
		return fmt.Errorf("cannot index execution result: %w", err)
	}

	err = s.myReceipts.BatchStoreMyReceipt(result.ExecutionReceipt, batch)
	if err != nil {
		return fmt.Errorf("could not persist execution result: %w", err)
	}

	// the state commitment is the last data item to be stored, so that
	// IsBlockExecuted can be implemented by checking whether state commitment exists
	// in the database
	err = s.commits.BatchStore(blockID, result.CurrentEndState(), batch)
	if err != nil {
		return fmt.Errorf("cannot store state commitment: %w", err)
	}

	err = batch.Flush()
	if err != nil {
		return fmt.Errorf("batch flush error: %w", err)
	}

	return nil
}

func (s *state) UpdateHighestExecutedBlockIfHigher(ctx context.Context, header *flow.Header) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEUpdateHighestExecutedBlockIfHigher)
		defer span.End()
	}

	return operation.RetryOnConflict(s.db.Update, procedure.UpdateHighestExecutedBlockIfHigher(header))
}

// deprecated by storehouse's GetHighestFinalizedExecuted
func (s *state) GetHighestExecutedBlockID(ctx context.Context) (uint64, flow.Identifier, error) {
	if s.enableRegisterStore {
		// when storehouse is enabled, the highest executed block is consisted as
		// the highest finalized and executed block
		height := s.GetHighestFinalizedExecuted()
		finalizedID, err := s.headers.BlockIDByHeight(height)
		if err != nil {
			return 0, flow.ZeroID, fmt.Errorf("could not get header by height %v: %w", height, err)
		}
		return height, finalizedID, nil
	}

	var blockID flow.Identifier
	var height uint64
	err := s.db.View(procedure.GetHighestExecutedBlock(&height, &blockID))
	if err != nil {
		return 0, flow.ZeroID, err
	}

	return height, blockID, nil
}

func (s *state) GetHighestFinalizedExecuted() uint64 {
	if !s.enableRegisterStore {
		// TODO: implement
		return 0
	}
	return s.registerStore.LastFinalizedAndExecutedHeight()
}

// IsBlockExecuted returns true if the block is executed, which means registers, events,
// results, etc are all stored.
// otherwise returns false
func (s *state) IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error) {
	if s.enableRegisterStore {
		return s.registerStore.IsBlockExecuted(height, blockID)
	}

	// ledger-based execution state uses commitment to determine if a block has been executed
	_, err := s.StateCommitmentByBlockID(blockID)

	// statecommitment exists means the block has been executed
	if err == nil {
		return true, nil
	}

	// statecommitment not exists means the block hasn't been executed yet
	if errors.Is(err, storage.ErrNotFound) {
		return false, nil
	}

	return false, err

}
