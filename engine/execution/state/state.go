package state

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution"
	fvmState "github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
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
	// NewStorageSnapshot creates a new ready-only view at the given state commitment.
	NewStorageSnapshot(flow.StateCommitment) fvmState.StorageSnapshot

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(context.Context, flow.Identifier) (flow.StateCommitment, error)

	// HasState returns true if the state with the given state commitment exists in memory
	HasState(flow.StateCommitment) bool

	// ChunkDataPackByChunkID retrieve a chunk data pack given the chunk ID.
	ChunkDataPackByChunkID(flow.Identifier) (*flow.ChunkDataPack, error)

	GetExecutionResultID(context.Context, flow.Identifier) (flow.Identifier, error)

	GetHighestExecutedBlockID(context.Context) (uint64, flow.Identifier, error)

	GetBlockIDByChunkID(chunkID flow.Identifier) (flow.Identifier, error)
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

const (
	KeyPartOwner = uint16(0)
	// @deprecated - controller was used only by the very first
	// version of cadence for access controll which was retired later on
	// KeyPartController = uint16(1)
	KeyPartKey = uint16(2)
)

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

func RegisterIDToKey(reg flow.RegisterID) ledger.Key {
	return ledger.NewKey([]ledger.KeyPart{
		ledger.NewKeyPart(KeyPartOwner, []byte(reg.Owner)),
		ledger.NewKeyPart(KeyPartKey, []byte(reg.Key)),
	})
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
		RegisterIDToKey(id),
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
		keys[i] = RegisterIDToKey(entry.Key)
		values[i] = entry.Value
	}
	return keys, values
}

// TODO(patrick): revisit caching.  readCache needs to be mutex guarded for
// parallel execution.
type LedgerStorageSnapshot struct {
	ledger     ledger.Ledger
	commitment flow.StateCommitment

	mutex     sync.RWMutex
	readCache map[flow.RegisterID]flow.RegisterValue // Guarded by mutex.
}

func NewLedgerStorageSnapshot(
	ldg ledger.Ledger,
	commitment flow.StateCommitment,
) fvmState.StorageSnapshot {
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
) fvmState.StorageSnapshot {
	return NewLedgerStorageSnapshot(s.ls, commitment)
}

type RegisterUpdatesHolder interface {
	UpdatedRegisters() flow.RegisterEntries
}

func CommitDelta(ldg ledger.Ledger, ruh RegisterUpdatesHolder, baseState flow.StateCommitment) (flow.StateCommitment, *ledger.TrieUpdate, error) {
	keys, values := RegisterEntriesToKeysValues(ruh.UpdatedRegisters())

	update, err := ledger.NewUpdate(ledger.State(baseState), keys, values)

	if err != nil {
		return flow.DummyStateCommitment, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	commit, trieUpdate, err := ldg.Set(update)
	if err != nil {
		return flow.DummyStateCommitment, nil, err
	}

	return flow.StateCommitment(commit), trieUpdate, nil
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

	header := result.ExecutableBlock.Block.Header
	blockID := header.ID()

	// Write Batch is BadgerDB feature designed for handling lots of writes
	// in efficient and automatic manner, hence pushing all the updates we can
	// as tightly as possible to let Badger manage it.
	// Note, that it does not guarantee atomicity as transactions has size limit,
	// but it's the closest thing to atomicity we could have
	batch := badgerstorage.NewBatch(s.db)

	for _, chunkDataPack := range result.AllChunkDataPacks() {
		err := s.chunkDataPacks.BatchStore(chunkDataPack, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}

		err = s.headers.BatchIndexByChunkID(blockID, chunkDataPack.ChunkID, batch)
		if err != nil {
			return fmt.Errorf("cannot index chunk data pack by blockID: %w", err)
		}
	}

	err := s.commits.BatchStore(blockID, result.CurrentEndState(), batch)
	if err != nil {
		return fmt.Errorf("cannot store state commitment: %w", err)
	}

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

	err = batch.Flush()
	if err != nil {
		return fmt.Errorf("batch flush error: %w", err)
	}

	//outside batch because it requires read access
	err = s.UpdateHighestExecutedBlockIfHigher(childCtx, header)
	if err != nil {
		return fmt.Errorf("cannot update highest executed block: %w", err)
	}
	return nil
}

func (s *state) GetBlockIDByChunkID(chunkID flow.Identifier) (flow.Identifier, error) {
	return s.headers.IDByChunkID(chunkID)
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

// IsBlockExecuted returns whether the block has been executed.
// it checks whether the state commitment exists in execution state.
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
