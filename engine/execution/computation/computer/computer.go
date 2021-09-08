package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/logging"
)

const SystemChunkEventCollectionMaxSize = 256_000_000 // ~256MB

// VirtualMachine runs procedures
type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
}

// ViewCommitter commits views's deltas to the ledger and collects the proofs
type ViewCommitter interface {
	// CommitView commits a views' register delta and collects proofs
	CommitView(state.View, flow.StateCommitment) (flow.StateCommitment, []byte, *ledger.TrieUpdate, error)
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(context.Context, *entity.ExecutableBlock, state.View, *programs.Programs) (*execution.ComputationResult, error)
}

type blockComputer struct {
	vm             VirtualMachine
	vmCtx          fvm.Context
	metrics        module.ExecutionMetrics
	tracer         module.Tracer
	log            zerolog.Logger
	systemChunkCtx fvm.Context
	committer      ViewCommitter
}

func SystemChunkContext(vmCtx fvm.Context, logger zerolog.Logger) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithRestrictedDeployment(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithServiceEventCollectionEnabled(),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(logger)),
		fvm.WithEventCollectionSizeLimit(SystemChunkEventCollectionMaxSize),
	)
}

// NewBlockComputer creates a new block executor.
func NewBlockComputer(
	vm VirtualMachine,
	vmCtx fvm.Context,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	logger zerolog.Logger,
	committer ViewCommitter,
) (BlockComputer, error) {
	return &blockComputer{
		vm:             vm,
		vmCtx:          vmCtx,
		metrics:        metrics,
		tracer:         tracer,
		log:            logger,
		systemChunkCtx: SystemChunkContext(vmCtx, logger),
		committer:      committer,
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	program *programs.Programs,
) (*execution.ComputationResult, error) {

	span, _, isSampled := e.tracer.StartBlockSpan(ctx, block.ID(), trace.EXEComputeBlock)
	if isSampled {
		span.SetTag("block.collectioncount", len(block.CompleteCollections))
		span.LogFields(log.String("block.hash", block.ID().String()))
	}
	defer span.Finish()

	results, err := e.executeBlock(span, block, stateView, program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	blockSpan opentracing.Span,
	block *entity.ExecutableBlock,
	stateView state.View,
	programs *programs.Programs,
) (*execution.ComputationResult, error) {

	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(block.Block.Header))
	systemChunkCtx := fvm.NewContextFromParent(e.systemChunkCtx, fvm.WithBlockHeader(block.Block.Header))
	collections := block.Collections()

	chunksSize := len(collections) + 1 // + 1 system chunk

	res := &execution.ComputationResult{
		ExecutableBlock:    block,
		Events:             make([]flow.EventsList, chunksSize),
		ServiceEvents:      make(flow.EventsList, 0),
		TransactionResults: make([]flow.TransactionResult, 0),
		StateCommitments:   make([]flow.StateCommitment, 0),
		Proofs:             make([][]byte, 0),
	}

	var txIndex uint32
	var err error
	var wg sync.WaitGroup
	wg.Add(2) // block commiter and event hasher

	stateCommitments := make([]flow.StateCommitment, 0, len(collections)+1)
	proofs := make([][]byte, 0, len(collections)+1)
	trieUpdates := make([]*ledger.TrieUpdate, len(collections)+1)

	bc := blockCommitter{
		committer: e.committer,
		blockSpan: blockSpan,
		tracer:    e.tracer,
		state:     *block.StartState,
		views:     make(chan state.View, len(collections)+1),
		callBack: func(state flow.StateCommitment, proof []byte, trieUpdate *ledger.TrieUpdate, err error) {
			if err != nil {
				panic(err)
			}
			stateCommitments = append(stateCommitments, state)
			proofs = append(proofs, proof)
			trieUpdates = append(trieUpdates, trieUpdate)
		},
	}

	eh := eventHasher{
		tracer:    e.tracer,
		data:      make(chan flow.EventsList, len(collections)+1),
		blockSpan: blockSpan,
		callBack: func(hash flow.Identifier, err error) {
			if err != nil {
				panic(err)
			}
			res.EventsHashes = append(res.EventsHashes, hash)
		},
	}

	go func() {
		bc.Run()
		wg.Done()
	}()

	go func() {
		eh.Run()
		wg.Done()
	}()

	collectionIndex := 0

	for i, collection := range collections {
		colView := stateView.NewChild()
		txIndex, err = e.executeCollection(blockSpan, collectionIndex, txIndex, blockCtx, colView, programs, collection, res)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}
		bc.Commit(colView)
		eh.Hash(res.Events[i])
		err = stateView.MergeView(colView)
		if err != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err)
		}
		collectionIndex++
	}

	// executing system chunk
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")
	colView := stateView.NewChild()
	_, err = e.executeSystemCollection(blockSpan, collectionIndex, txIndex, systemChunkCtx, colView, programs, res)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}
	bc.Commit(colView)
	eh.Hash(res.Events[len(res.Events)-1])
	err = stateView.MergeView(colView)
	if err != nil {
		return nil, fmt.Errorf("cannot merge view: %w", err)
	}

	// close the views and wait for all views to be committed
	close(bc.views)
	close(eh.data)
	wg.Wait()
	res.StateReads = stateView.(*delta.View).ReadsCount()
	res.StateCommitments = stateCommitments
	res.Proofs = proofs
	res.TrieUpdates = trieUpdates

	return res, nil
}

func (e *blockComputer) executeSystemCollection(
	blockSpan opentracing.Span,
	collectionIndex int,
	txIndex uint32,
	systemChunkCtx fvm.Context,
	collectionView state.View,
	programs *programs.Programs,
	res *execution.ComputationResult,
) (uint32, error) {

	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeSystemCollection)
	defer colSpan.Finish()

	tx, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return txIndex, fmt.Errorf("could not get system chunk transaction: %w", err)
	}

	err = e.executeTransaction(tx, colSpan, collectionView, programs, systemChunkCtx, collectionIndex, txIndex, res)
	txIndex++

	if err != nil {
		return txIndex, err
	}

	systemChunkTxResult := res.TransactionResults[len(res.TransactionResults)-1]
	if systemChunkTxResult.ErrorMessage != "" {
		// This log is used as the data source for an alert on grafana.
		// The system_chunk_error field must not be changed without adding the corresponding
		// changes in grafana. https://github.com/dapperlabs/flow-internal/issues/1546
		e.log.Error().
			Str("error_message", systemChunkTxResult.ErrorMessage).
			Hex("block_id", logging.Entity(systemChunkCtx.BlockHeader)).
			Bool("system_chunk_error", true).
			Bool("critical_error", true).
			Msg("error executing system chunk transaction")
	}

	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())

	return txIndex, err
}

func (e *blockComputer) executeCollection(
	blockSpan opentracing.Span,
	collectionIndex int,
	txIndex uint32,
	blockCtx fvm.Context,
	collectionView state.View,
	programs *programs.Programs,
	collection *entity.CompleteCollection,
	res *execution.ComputationResult,
) (uint32, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(blockCtx.BlockHeader)).
		Hex("collection_id", logging.Entity(collection.Guarantee)).
		Msg("executing collection")

	// call tracing
	startedAt := time.Now()
	computationUsedUpToNow := res.ComputationUsed
	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeCollection)
	defer func() {
		colSpan.SetTag("collection.txCount", len(collection.Transactions))
		colSpan.LogFields(
			log.String("collection.hash", collection.Guarantee.CollectionID.String()),
		)
		colSpan.Finish()
	}()

	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsReporter(e.metrics), fvm.WithTracer(e.tracer))
	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(txBody, colSpan, collectionView, programs, txCtx, collectionIndex, txIndex, res)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}
	res.AddStateSnapshot(collectionView.(*delta.View).Interactions())
	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("blockID", collection.Guarantee.ReferenceBlockID.String()).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt), res.ComputationUsed-computationUsedUpToNow, len(collection.Transactions))

	return txIndex, nil
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan opentracing.Span,
	collectionView state.View,
	programs *programs.Programs,
	ctx fvm.Context,
	collectionIndex int,
	txIndex uint32,
	res *execution.ComputationResult,
) error {
	startedAt := time.Now()
	txID := txBody.ID()

	// we capture two spans one for tx-based view and one for the current context (block-based) view
	txSpan := e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)
	txSpan.LogFields(log.String("transaction.ID", txID.String()))
	defer txSpan.Finish()

	txInternalSpan, _, isSampled := e.tracer.StartTransactionSpan(context.Background(), txID, trace.EXERunTransaction)
	if isSampled {
		txInternalSpan.LogFields(log.String("transaction.ID", txID.String()))
	}
	defer txInternalSpan.Finish()

	e.log.Debug().
		Hex("tx_id", logging.Entity(txBody)).
		Msg("executing transaction")

	tx := fvm.Transaction(txBody, txIndex)
	tx.SetTraceSpan(txInternalSpan)

	txView := collectionView.NewChild()
	err := e.vm.Run(ctx, tx, txView, programs)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	txResult := flow.TransactionResult{
		TransactionID:   tx.ID,
		ComputationUsed: tx.ComputationUsed,
	}

	if tx.Err != nil {
		txResult.ErrorMessage = tx.Err.Error()
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Str("error_message", tx.Err.Error()).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction execution failed")
	} else {
		e.log.Debug().
			Hex("tx_id", logging.Entity(txBody)).
			Msg("transaction executed successfully")
	}

	mergeSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEMergeTransactionView)
	defer mergeSpan.Finish()

	// always merge the view, fvm take cares of reverting changes
	// of failed transaction invocation
	err = collectionView.MergeView(txView)
	if err != nil {
		return fmt.Errorf("merging tx view to collection view failed: %w", err)
	}

	res.AddEvents(collectionIndex, tx.Events)
	res.AddServiceEvents(tx.ServiceEvents)
	res.AddTransactionResult(&txResult)
	res.AddComputationUsed(tx.ComputationUsed)

	e.log.Info().
		Str("txHash", tx.ID.String()).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("transaction executed")

	e.metrics.ExecutionTransactionExecuted(time.Since(startedAt), tx.ComputationUsed, len(tx.Events), tx.Err != nil)
	return nil
}

type blockCommitter struct {
	tracer    module.Tracer
	committer ViewCommitter
	callBack  func(state flow.StateCommitment, proof []byte, update *ledger.TrieUpdate, err error)
	state     flow.StateCommitment
	views     chan state.View
	blockSpan opentracing.Span
}

func (bc *blockCommitter) Run() {
	for view := range bc.views {
		span := bc.tracer.StartSpanFromParent(bc.blockSpan, trace.EXECommitDelta)
		stateCommit, proof, trieUpdate, err := bc.committer.CommitView(view, bc.state)
		bc.callBack(stateCommit, proof, trieUpdate, err)
		bc.state = stateCommit
		span.Finish()
	}
}

func (bc *blockCommitter) Commit(view state.View) {
	bc.views <- view
}

type eventHasher struct {
	tracer    module.Tracer
	callBack  func(hash flow.Identifier, err error)
	data      chan flow.EventsList
	blockSpan opentracing.Span
}

func (eh *eventHasher) Run() {
	for data := range eh.data {
		span := eh.tracer.StartSpanFromParent(eh.blockSpan, trace.EXEHashEvents)
		data, err := flow.EventsListHash(data)
		eh.callBack(data, err)
		span.Finish()
	}
}

func (eh *eventHasher) Hash(events flow.EventsList) {
	eh.data <- events
}
