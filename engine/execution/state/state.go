package state

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
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

// TODO Many operations here are should be transactional, so we need to refactor this
// to store a reference to DB and compose operations and procedures rather then
// just being amalgamate of proxies for single transactions operation

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	ReadOnlyExecutionState

	// CommitDelta commits a register delta and returns the new state commitment.
	CommitDelta(context.Context, delta.Delta, flow.StateCommitment) (flow.StateCommitment, error)

	// PersistStateCommitment saves a state commitment by the given block ID.
	PersistStateCommitment(context.Context, flow.Identifier, flow.StateCommitment) error

	// PersistChunkDataPack stores a chunk data pack by chunk ID.
	PersistChunkDataPack(context.Context, *flow.ChunkDataPack) error

	PersistExecutionResult(ctx context.Context, result *flow.ExecutionResult) error

	PersistExecutionReceipt(context.Context, *flow.ExecutionReceipt) error

	PersistStateInteractions(context.Context, flow.Identifier, []*delta.Snapshot) error

	UpdateHighestExecutedBlockIfHigher(context.Context, *flow.Header) error

	RemoveByBlockID(flow.Identifier) error

	SetHighestExecuted(*flow.Header) error
}

const (
	KeyPartOwner      = uint16(0)
	KeyPartController = uint16(1)
	KeyPartKey        = uint16(2)
)

type state struct {
	tracer         module.Tracer
	ls             ledger.Ledger
	commits        storage.Commits
	blocks         storage.Blocks
	collections    storage.Collections
	chunkDataPacks storage.ChunkDataPacks
	results        storage.ExecutionResults
	receipts       storage.ExecutionReceipts
	db             *badger.DB
}

func (s *state) PersistExecutionResult(ctx context.Context, executionResult *flow.ExecutionResult) error {

	err := s.results.Store(executionResult)
	if err != nil {
		return fmt.Errorf("could not store result: %w", err)
	}

	err = s.results.Index(executionResult.BlockID, executionResult.ID())
	if err != nil {
		return fmt.Errorf("could not index execution result: %w", err)
	}
	return nil
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
	collections storage.Collections,
	chunkDataPacks storage.ChunkDataPacks,
	results storage.ExecutionResults,
	receipts storage.ExecutionReceipts,
	db *badger.DB,
	tracer module.Tracer,
) ExecutionState {
	return &state{
		tracer:         tracer,
		ls:             ls,
		commits:        commits,
		blocks:         blocks,
		collections:    collections,
		chunkDataPacks: chunkDataPacks,
		results:        results,
		receipts:       receipts,
		db:             db,
	}

}

func makeSingleValueQuery(commitment flow.StateCommitment, owner, controller, key string) (*ledger.Query, error) {
	return ledger.NewQuery(commitment,
		[]ledger.Key{
			RegisterIDToKey(flow.NewRegisterID(owner, controller, key)),
		})
}

func makeQuery(commitment flow.StateCommitment, ids []flow.RegisterID) (*ledger.Query, error) {

	keys := make([]ledger.Key, len(ids))
	for i, id := range ids {
		keys[i] = RegisterIDToKey(id)
	}

	return ledger.NewQuery(commitment, keys)
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
	return func(owner, controller, key string) (flow.RegisterValue, error) {

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

		return values[0], nil
	}
}

func (s *state) NewView(commitment flow.StateCommitment) *delta.View {
	return delta.NewView(LedgerGetRegister(s.ls, commitment))
}

func CommitDelta(ldg ledger.Ledger, delta delta.Delta, baseState flow.StateCommitment) (flow.StateCommitment, error) {
	ids, values := delta.RegisterUpdates()

	update, err := ledger.NewUpdate(
		baseState,
		RegisterIDSToKeys(ids),
		RegisterValuesToValues(values),
	)

	if err != nil {
		return nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	commit, err := ldg.Set(update)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func (s *state) CommitDelta(ctx context.Context, delta delta.Delta, baseState flow.StateCommitment) (flow.StateCommitment, error) {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXECommitDelta)
		defer span.Finish()
	}

	return CommitDelta(s.ls, delta, baseState)
}

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
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetRegisters)
		defer span.Finish()
	}

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

	query, err := makeQuery(commit, registerIDs)

	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	proof, err := s.ls.Prove(query)
	if err != nil {
		return nil, fmt.Errorf("cannot get proof: %w", err)
	}
	return proof, nil
}

func (s *state) StateCommitmentByBlockID(ctx context.Context, blockID flow.Identifier) (flow.StateCommitment, error) {
	return s.commits.ByBlockID(blockID)
}

func (s *state) PersistStateCommitment(ctx context.Context, blockID flow.Identifier, commit flow.StateCommitment) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEPersistStateCommitment)
		defer span.Finish()
	}

	return s.commits.Store(blockID, commit)
}

func (s *state) ChunkDataPackByChunkID(ctx context.Context, chunkID flow.Identifier) (*flow.ChunkDataPack, error) {
	span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEPersistStateCommitment)
	defer span.Finish()

	return s.chunkDataPacks.ByChunkID(chunkID)
}

func (s *state) PersistChunkDataPack(ctx context.Context, c *flow.ChunkDataPack) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEPersistChunkDataPack)
		defer span.Finish()
	}

	return s.chunkDataPacks.Store(c)
}

func (s *state) GetExecutionResultID(ctx context.Context, blockID flow.Identifier) (flow.Identifier, error) {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEGetExecutionResultID)
		defer span.Finish()
	}

	result, err := s.results.ByBlockID(blockID)
	if err != nil {
		return flow.ZeroID, err
	}
	return result.ID(), nil
}

func (s *state) PersistExecutionReceipt(ctx context.Context, receipt *flow.ExecutionReceipt) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEPersistExecutionResult)
		defer span.Finish()
	}

	err := s.receipts.Store(receipt)
	if err != nil {
		return fmt.Errorf("could not persist execution result: %w", err)
	}
	// TODO if the second operation fails we should remove stored execution result
	// This is global execution storage problem - see TODO at the top
	err = s.receipts.Index(receipt.ExecutionResult.BlockID, receipt.ID())
	if err != nil {
		return fmt.Errorf("could not index execution receipt: %w", err)
	}
	return nil
}

func (s *state) PersistStateInteractions(ctx context.Context, blockID flow.Identifier, views []*delta.Snapshot) error {
	if s.tracer != nil {
		span, _ := s.tracer.StartSpanFromContext(ctx, trace.EXEPersistStateInteractions)
		defer span.Finish()
	}

	return operation.RetryOnConflict(s.db.Update, operation.InsertExecutionStateInteractions(blockID, views))
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
			StartState:          startStateCommitment,
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

func (s *state) RemoveByBlockID(blockID flow.Identifier) error {
	err := s.receipts.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove receipt: %w", err)
	}

	err = s.commits.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove commits: %w", err)
	}

	err = s.results.RemoveByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not remove result: %w", err)
	}

	return nil
}

func (s *state) SetHighestExecuted(header *flow.Header) error {
	return operation.RetryOnConflict(s.db.Update, func(txn *badger.Txn) error {
		err := operation.UpdateExecutedBlock(header.ID())(txn)
		if err != nil {
			return fmt.Errorf("cannot set highest executed block: %w", err)
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
