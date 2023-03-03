package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/storage/badger/procedure"
)

// SnapshotReader provides read functionality at a specific snapshot
//
// TODO(ramtin): update this interface with ReadByPrefix and Warmup functionality
// TODO(ramtin): maybe consolidate this with the FVM one
type SnapshotReader interface {
	// Read returns the value for a single register
	// if no value is found it returns an empty byte slice and no error
	// returned errors are fatal
	// TODO(ramtin): rename to Read
	Get(id flow.RegisterID) (value flow.RegisterValue, err error)

	// BatchRead returns a set of values for the given registerIDs
	// it has the same behaviour as `Read` except it operates on a batch of registers
	// returned errors are fatal
	BatchRead(ids []flow.RegisterID) (itr flow.RegisterIterator, err error)
}

type BlockAwareStorage interface {
	// OnFinalizedBlock is called whenever a block has been finalized.
	// Calls are in the order the blocks are finalized.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnFinalizedBlock(block *flow.Header)

	// OnSealedBlock is called when a block is sealed
	// Calls are in the order blocks get sealed.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnSealedBlock(block *flow.Header)

	// OnBlockExecuted is called when a block is executed
	// Calls are in the order blocks get executed.
	// Prerequisites:
	// Implementation must be concurrency safe;
	// and must handle repetition of the same events (with some processing overhead).
	OnBlockExecuted(block *flow.Header, delta delta.SpockSnapshot) error
}

type ReadOnlyRegisterStorage interface {
	module.ReadyDoneAware
	// SnapshotReaderAtBlock returns an SnapshotReader for the given blockID
	// if a register is not changed in the given block the latest value from ancestor blocks is returned
	// it could be used for reading several values of the same blockID
	SnapshotReaderAtBlock(blockID flow.Identifier) (SnapshotReader, error)

	// TODO(ramtin): consolidate this into above and depricate
	NewStorageSnapshot(commitment flow.StateCommitment) SnapshotReader
}

// RegisterStorage stores registers at given height,
type RegisterStorage interface {
	ReadOnlyRegisterStorage
	BlockAwareStorage
}

type ReadOnlyAttestationStorage interface {
	module.ReadyDoneAware

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(context.Context, flow.Identifier) (flow.StateCommitment, error)

	// HasState returns true if the state with the given state commitment exists in memory
	HasState(flow.StateCommitment) bool

	// ChunkDataPackByChunkID retrieve a chunk data pack given the chunk ID.
	ChunkDataPackByChunkID(flow.Identifier) (*flow.ChunkDataPack, error)

	// GetExecutionResultID returns execution result ID for the given blockID
	GetExecutionResultID(context.Context, flow.Identifier) (flow.Identifier, error)

	// GetHighestExecutedBlockID returns the highest executed block height and blockID
	// TODO(ramtin): this probably should be captured in another interface controlled by ingestion engine
	GetHighestExecutedBlockID(context.Context) (uint64, flow.Identifier, error)

	// GetBlockIDByChunkID returns the blockID for the given ChunkID
	// TODO(ramtin): we probably don't need this
	GetBlockIDByChunkID(chunkID flow.Identifier) (flow.Identifier, error)
}

// AttestationStorage stores attestations needed to prove execution
//
// TODO(ramtin): clean up some of the endpoints and move them to their relevant interface
type AttestationStorage interface {
	ReadOnlyAttestationStorage

	BlockAwareStorage

	UpdateHighestExecutedBlockIfHigher(context.Context, *flow.Header) error

	SaveExecutionResults(
		ctx context.Context,
		result *execution.ComputationResult,
		executionReceipt *flow.ExecutionReceipt,
	) error
}

// ReadOnlyExecutionState allows to read the execution state
type ReadOnlyExecutionState interface {
	ReadOnlyRegisterStorage
	ReadOnlyAttestationStorage
}

// TODO Many operations here are should be transactional, so we need to refactor this
// to store a reference to DB and compose operations and procedures rather then
// just being amalgamate of proxies for single transactions operation

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	ReadOnlyExecutionState
	RegisterStorage
	AttestationStorage
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
	// TODO(ramtin): properly implement the read done methods
	module.NoopReadyDoneAware
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

	readCache map[flow.RegisterID]flow.RegisterValue
}

func NewLedgerStorageSnapshot(
	ldg ledger.Ledger,
	commitment flow.StateCommitment,
) SnapshotReader {
	return &LedgerStorageSnapshot{
		ledger:     ldg,
		commitment: commitment,
		readCache:  make(map[flow.RegisterID]flow.RegisterValue),
	}
}

func (storage *LedgerStorageSnapshot) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	if value, ok := storage.readCache[id]; ok {
		return value, nil
	}

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

	// Prevent caching of value with len zero
	if len(value) == 0 {
		return nil, nil
	}

	// don't cache value with len zero
	storage.readCache[id] = value

	return value, nil
}

// TODO(ramtin): implement me
func (*LedgerStorageSnapshot) BatchRead(ids []flow.RegisterID) (itr flow.RegisterIterator, err error) {
	return nil, errors.New("not implemented yet")
}

func (s *state) SnapshotReaderAtBlock(blockID flow.Identifier) (SnapshotReader, error) {
	stateCommit, err := s.commits.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot get the state commitment for block %s: %w",
			blockID.String(),
			err)
	}
	return NewLedgerStorageSnapshot(s.ls, stateCommit), nil
}

func (s *state) NewStorageSnapshot(commitment flow.StateCommitment) SnapshotReader {
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
	executionReceipt *flow.ExecutionReceipt,
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

	for _, chunkDataPack := range result.ChunkDataPacks {
		err := s.chunkDataPacks.BatchStore(chunkDataPack, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}

		err = s.headers.BatchIndexByChunkID(blockID, chunkDataPack.ChunkID, batch)
		if err != nil {
			return fmt.Errorf("cannot index chunk data pack by blockID: %w", err)
		}
	}

	err := s.commits.BatchStore(blockID, result.EndState, batch)
	if err != nil {
		return fmt.Errorf("cannot store state commitment: %w", err)
	}

	err = s.events.BatchStore(blockID, result.Events, batch)
	if err != nil {
		return fmt.Errorf("cannot store events: %w", err)
	}

	err = s.serviceEvents.BatchStore(blockID, result.ServiceEvents, batch)
	if err != nil {
		return fmt.Errorf("cannot store service events: %w", err)
	}

	err = s.transactionResults.BatchStore(
		blockID,
		result.TransactionResults,
		batch)
	if err != nil {
		return fmt.Errorf("cannot store transaction result: %w", err)
	}

	executionResult := &executionReceipt.ExecutionResult
	err = s.results.BatchStore(executionResult, batch)
	if err != nil {
		return fmt.Errorf("cannot store execution result: %w", err)
	}

	err = s.results.BatchIndex(blockID, executionResult.ID(), batch)
	if err != nil {
		return fmt.Errorf("cannot index execution result: %w", err)
	}

	err = s.myReceipts.BatchStoreMyReceipt(executionReceipt, batch)
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

func (s *state) OnFinalizedBlock(block *flow.Header) {
	// no-op
	return
}

func (s *state) OnSealedBlock(block *flow.Header) {
	// no-op
	return
}

func (s *state) OnBlockExecuted(block *flow.Header, delta delta.SpockSnapshot) error {
	// ramtin: currently we don't do anything here, the committer injected to the computer engine is doing the update right now
	// no-op
	return nil
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
