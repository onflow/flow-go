package state

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution"
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

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(context.Context, flow.Identifier) (flow.StateCommitment, error)

	// HasState returns true if the state with the given state commitment exists in memory
	HasState(flow.StateCommitment) bool
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
) ExecutionState {
	return &state{
		tracer:             tracer,
		ls:                 ls,
		commits:            commits,
		blocks:             blocks,
		headers:            headers,
		collections:        collections,
		chunkDataPacks:     chunkDataPacks,
		results:            results,
		myReceipts:         myReceipts,
		events:             events,
		serviceEvents:      serviceEvents,
		transactionResults: transactionResults,
		db:                 db,
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
	return NewLedgerStorageSnapshot(s.ls, commitment)
}

type RegisterUpdatesHolder interface {
	UpdatedRegisters() flow.RegisterEntries
	UpdatedRegisterSet() map[flow.RegisterID]flow.RegisterValue
}

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

func (s *state) HasState(commitment flow.StateCommitment) bool {
	return s.ls.HasState(ledger.State(commitment))
}

func (s *state) StateCommitmentByBlockID(ctx context.Context, blockID flow.Identifier) (flow.StateCommitment, error) {
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

func (s *state) GetHighestExecutedBlockID(ctx context.Context) (uint64, flow.Identifier, error) {
	var blockID flow.Identifier
	var height uint64
	err := s.db.View(procedure.GetHighestExecutedBlock(&height, &blockID))
	if err != nil {
		return 0, flow.ZeroID, err
	}

	return height, blockID, nil
}

// IsBlockExecuted returns true if the block is executed, which means registers, events,
// results, statecommitment etc are all stored.
// otherwise returns false
func IsBlockExecuted(ctx context.Context, state ReadOnlyExecutionState, block flow.Identifier) (bool, error) {
	_, err := state.StateCommitmentByBlockID(ctx, block)

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
