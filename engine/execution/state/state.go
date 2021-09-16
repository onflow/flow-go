package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/storage"
	badgerstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

// ReadOnlyExecutionState allows to read the execution state
type ReadOnlyExecutionState interface {
	// NewView creates a new ready-only view at the given state commitment.
	NewView(flow.StateCommitment) *delta.View

	GetRegisters(
		context.Context,
		flow.StateCommitment,
		[]flow.RegisterID,
	) ([]flow.RegisterValue, error)

	GetProof(
		context.Context,
		flow.StateCommitment,
		[]flow.RegisterID,
	) (flow.StorageProof, error)

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(context.Context, flow.Identifier) (flow.StateCommitment, error)

	// ChunkDataPackByChunkID retrieve a chunk data pack given the chunk ID.
	ChunkDataPackByChunkID(context.Context, flow.Identifier) (*flow.ChunkDataPack, error)

	GetExecutionResultID(context.Context, flow.Identifier) (flow.Identifier, error)

	RetrieveStateDelta(context.Context, flow.Identifier) (*messages.ExecutionStateDelta, error)

	GetHighestExecutedBlockID(context.Context) (uint64, flow.Identifier, error)

	GetCollection(identifier flow.Identifier) (*flow.Collection, error)

	GetBlockIDByChunkID(chunkID flow.Identifier) (flow.Identifier, error)
}

// TODO Many operations here are should be transactional, so we need to refactor this
// to store a reference to DB and compose operations and procedures rather then
// just being amalgamate of proxies for single transactions operation

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	ReadOnlyExecutionState

	UpdateHighestExecutedBlockIfHigher(context.Context, *flow.Header) error

	PersistExecutionState(ctx context.Context, header *flow.Header, endState flow.StateCommitment,
		chunkDataPacks []*flow.ChunkDataPack,
		executionReceipt *flow.ExecutionReceipt, events []flow.EventsList, serviceEvents flow.EventsList, results []flow.TransactionResult) error
}

const (
	KeyPartOwner      = uint16(0)
	KeyPartController = uint16(1)
	KeyPartKey        = uint16(2)
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
	receipts           storage.ExecutionReceipts
	myReceipts         storage.MyExecutionReceipts
	events             storage.Events
	serviceEvents      storage.ServiceEvents
	transactionResults storage.TransactionResults
	db                 *badger.DB
}

func RegisterIDToKey(reg flow.RegisterID) ledger.Key {
	return ledger.NewKey([]ledger.KeyPart{
		ledger.NewKeyPart(KeyPartOwner, []byte(reg.Owner)),
		ledger.NewKeyPart(KeyPartController, []byte(reg.Controller)),
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
	receipts storage.ExecutionReceipts,
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
		receipts:           receipts,
		myReceipts:         myReceipts,
		events:             events,
		serviceEvents:      serviceEvents,
		transactionResults: transactionResults,
		db:                 db,
	}

}

func makeSingleValueQuery(commitment flow.StateCommitment, owner, controller, key string) (*ledger.Query, error) {
	return ledger.NewQuery(ledger.State(commitment),
		[]ledger.Key{
			RegisterIDToKey(flow.NewRegisterID(owner, controller, key)),
		})
}

func makeQuery(commitment flow.StateCommitment, ids []flow.RegisterID) (*ledger.Query, error) {

	keys := make([]ledger.Key, len(ids))
	for i, id := range ids {
		keys[i] = RegisterIDToKey(id)
	}

	return ledger.NewQuery(ledger.State(commitment), keys)
}

func RegisterIDSToKeys(ids []flow.RegisterID) []ledger.Key {
	keys := make([]ledger.Key, len(ids))
	for i, id := range ids {
		keys[i] = RegisterIDToKey(id)
	}
	return keys
}

func RegisterValuesToValues(values []flow.RegisterValue) []ledger.Value {
	vals := make([]ledger.Value, len(values))
	for i, value := range values {
		vals[i] = value
	}
	return vals
}

func LedgerGetRegister(ldg ledger.Ledger, commitment flow.StateCommitment) delta.GetRegisterFunc {

	readCache := make(map[flow.RegisterID]flow.RegisterEntry)

	return func(owner, controller, key string) (flow.RegisterValue, error) {
		regID := flow.RegisterID{
			Owner:      owner,
			Controller: controller,
			Key:        key,
		}

		if value, ok := readCache[regID]; ok {
			return value.Value, nil
		}

		query, err := makeSingleValueQuery(commitment, owner, controller, key)

		if err != nil {
			return nil, fmt.Errorf("cannot create ledger query: %w", err)
		}

		values, err := ldg.Get(query)

		if err != nil {
			return nil, fmt.Errorf("error getting register (%s) value at %x: %w", key, commitment, err)
		}

		if len(values) == 0 {
			return nil, nil
		}

		// don't cache value with len zero
		readCache[regID] = flow.RegisterEntry{Key: regID, Value: values[0]}

		return values[0], nil
	}
}

func (s *state) NewView(commitment flow.StateCommitment) *delta.View {
	return delta.NewView(LedgerGetRegister(s.ls, commitment))
}

type RegisterUpdatesHolder interface {
	RegisterUpdates() ([]flow.RegisterID, []flow.RegisterValue)
}

func CommitDelta(ldg ledger.Ledger, ruh RegisterUpdatesHolder, baseState flow.StateCommitment) (flow.StateCommitment, *ledger.TrieUpdate, error) {
	ids, values := ruh.RegisterUpdates()

	update, err := ledger.NewUpdate(
		ledger.State(baseState),
		RegisterIDSToKeys(ids),
		RegisterValuesToValues(values),
	)

	if err != nil {
		return flow.DummyStateCommitment, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	commit, trieUpdate, err := ldg.Set(update)
	if err != nil {
		return flow.DummyStateCommitment, nil, err
	}

	return flow.StateCommitment(commit), trieUpdate, nil
}

//func (s *state) CommitDelta(ctx context.Context, delta delta.Delta, baseState flow.StateCommitment) (flow.StateCommitment, error) {
//	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXECommitDelta)
//	defer span.Finish()
//
//	return CommitDelta(s.ls, delta, baseState)
//}

func (s *state) getRegisters(commit flow.StateCommitment, registerIDs []flow.RegisterID) (*ledger.Query, []ledger.Value, error) {

	query, err := makeQuery(commit, registerIDs)

	if err != nil {
		return nil, nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	values, err := s.ls.Get(query)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot query ledger: %w", err)
	}

	return query, values, err
}

func (s *state) GetRegisters(
	ctx context.Context,
	commit flow.StateCommitment,
	registerIDs []flow.RegisterID,
) ([]flow.RegisterValue, error) {
	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetRegisters)
	defer span.Finish()

	_, values, err := s.getRegisters(commit, registerIDs)
	if err != nil {
		return nil, err
	}

	registerValues := make([]flow.RegisterValue, len(values))
	for i, v := range values {
		registerValues[i] = v
	}

	return registerValues, nil
}

func (s *state) GetProof(
	ctx context.Context,
	commit flow.StateCommitment,
	registerIDs []flow.RegisterID,
) (flow.StorageProof, error) {

	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetRegistersWithProofs)
	defer span.Finish()

	query, err := makeQuery(commit, registerIDs)

	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	// Get proofs in an arbitrary order, not correlated to the register ID order in the query.
	proof, err := s.ls.Prove(query)
	if err != nil {
		return nil, fmt.Errorf("cannot get proof: %w", err)
	}
	return proof, nil
}

func (s *state) StateCommitmentByBlockID(ctx context.Context, blockID flow.Identifier) (flow.StateCommitment, error) {
	return s.commits.ByBlockID(blockID)
}

func (s *state) ChunkDataPackByChunkID(ctx context.Context, chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	chunkDataPack, err := s.chunkDataPacks.ByChunkID(chunkID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve stored chunk data pack: %w", err)
	}

	return chunkDataPack, nil
}

func (s *state) GetExecutionResultID(ctx context.Context, blockID flow.Identifier) (flow.Identifier, error) {
	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetExecutionResultID)
	defer span.Finish()

	result, err := s.results.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, err
	}
	return result.ID(), nil
}

func (s *state) PersistExecutionState(ctx context.Context, header *flow.Header, endState flow.StateCommitment,
	chunkDataPacks []*flow.ChunkDataPack, executionReceipt *flow.ExecutionReceipt, events []flow.EventsList, serviceEvents flow.EventsList,
	results []flow.TransactionResult) error {

	spew.Config.DisableMethods = true
	spew.Config.DisablePointerMethods = true

	span, childCtx := s.tracer.StartSpanFromContext(ctx, trace.EXEStatePersistExecutionState)
	defer span.Finish()

	blockID := header.ID()

	// Write Batch is BadgerDB feature designed for handling lots of writes
	// in efficient and automatic manner, hence pushing all the updates we can
	// as tightly as possible to let Badger manage it.
	// Note, that it does not guarantee atomicity as transactions has size limit
	// but it's the closes thing to atomicity we could have
	batch := badgerstorage.NewBatch(s.db)

	for _, chunkDataPack := range chunkDataPacks {
		err := s.chunkDataPacks.BatchStore(chunkDataPack, batch)
		if err != nil {
			return fmt.Errorf("cannot store chunk data pack: %w", err)
		}

		err = s.headers.BatchIndexByChunkID(header.ID(), chunkDataPack.ChunkID, batch)
		if err != nil {
			return fmt.Errorf("cannot index chunk data pack by blockID: %w", err)
		}
	}

	err := s.commits.BatchStore(blockID, endState, batch)
	if err != nil {
		return fmt.Errorf("cannot store state commitment: %w", err)
	}

	err = s.events.BatchStore(blockID, events, batch)
	if err != nil {
		return fmt.Errorf("cannot store events: %w", err)
	}

	err = s.serviceEvents.BatchStore(blockID, serviceEvents, batch)
	if err != nil {
		return fmt.Errorf("cannot store service events: %w", err)
	}

	err = s.transactionResults.BatchStore(blockID, results, batch)
	if err != nil {
		return fmt.Errorf("cannot store transaction result: %w", err)
	}

	executionResult := &executionReceipt.ExecutionResult
	err = s.results.BatchStore(executionResult, batch)
	if err != nil {
		return fmt.Errorf("cannot store execution result: %w", err)
	}

	// it overwrites the index if exists already
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

func (s *state) RetrieveStateDelta(ctx context.Context, blockID flow.Identifier) (*messages.ExecutionStateDelta, error) {
	block, err := s.blocks.ByID(blockID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve block: %w", err)
	}
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)

	for _, guarantee := range block.Payload.Guarantees {
		collection, err := s.collections.ByID(guarantee.CollectionID)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve collection for delta: %w", err)
		}
		completeCollections[collection.ID()] = &entity.CompleteCollection{
			Guarantee:    guarantee,
			Transactions: collection.Transactions,
		}
	}

	var startStateCommitment flow.StateCommitment
	var endStateCommitment flow.StateCommitment
	var stateInteractions []*delta.Snapshot
	var events []flow.Event
	var serviceEvents []flow.Event
	var txResults []flow.TransactionResult

	err = s.db.View(func(txn *badger.Txn) error {
		err = operation.LookupStateCommitment(blockID, &endStateCommitment)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup state commitment: %w", err)

		}

		err = operation.LookupStateCommitment(block.Header.ParentID, &startStateCommitment)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup parent state commitment: %w", err)
		}

		err = operation.LookupEventsByBlockID(blockID, &events)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup events: %w", err)
		}

		err = operation.LookupServiceEventsByBlockID(blockID, &serviceEvents)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup events: %w", err)
		}

		err = operation.LookupTransactionResultsByBlockID(blockID, &txResults)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup transaction errors: %w", err)
		}

		err = operation.RetrieveExecutionStateInteractions(blockID, &stateInteractions)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup execution state views: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &messages.ExecutionStateDelta{
		ExecutableBlock: entity.ExecutableBlock{
			Block:               block,
			StartState:          &startStateCommitment,
			CompleteCollections: completeCollections,
		},
		StateInteractions:  stateInteractions,
		EndState:           endStateCommitment,
		Events:             events,
		ServiceEvents:      serviceEvents,
		TransactionResults: txResults,
	}, nil
}

func (s *state) GetCollection(identifier flow.Identifier) (*flow.Collection, error) {
	return s.collections.ByID(identifier)
}

func (s *state) GetBlockIDByChunkID(chunkID flow.Identifier) (flow.Identifier, error) {
	return s.headers.IDByChunkID(chunkID)
}

func (s *state) UpdateHighestExecutedBlockIfHigher(ctx context.Context, header *flow.Header) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEUpdateHighestExecutedBlockIfHigher)
		defer span.Finish()
	}

	return operation.RetryOnConflict(s.db.Update, func(txn *badger.Txn) error {
		var blockID flow.Identifier
		err := operation.RetrieveExecutedBlock(&blockID)(txn)
		if err != nil {
			return fmt.Errorf("cannot lookup executed block: %w", err)
		}

		var highest flow.Header
		err = operation.RetrieveHeader(blockID, &highest)(txn)
		if err != nil {
			return fmt.Errorf("cannot retrieve executed header: %w", err)
		}

		if header.Height <= highest.Height {
			return nil
		}
		err = operation.UpdateExecutedBlock(header.ID())(txn)
		if err != nil {
			return fmt.Errorf("cannot update highest executed block: %w", err)
		}

		return nil
	})
}

func (s *state) GetHighestExecutedBlockID(ctx context.Context) (uint64, flow.Identifier, error) {
	var blockID flow.Identifier
	var highest flow.Header
	err := s.db.View(func(tx *badger.Txn) error {
		err := operation.RetrieveExecutedBlock(&blockID)(tx)
		if err != nil {
			return fmt.Errorf("could not lookup executed block: %w", err)
		}
		err = operation.RetrieveHeader(blockID, &highest)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve executed header: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, flow.ZeroID, err
	}

	return highest.Height, blockID, nil
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
