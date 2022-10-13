package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/debug"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	SystemChunkEventCollectionMaxSize = 256_000_000 // ~256MB
)

// VirtualMachine runs procedures
type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View) error
	GetAccount(fvm.Context, flow.Address, state.View) (*flow.Account, error)
}

// ViewCommitter commits views's deltas to the ledger and collects the proofs
type ViewCommitter interface {
	// CommitView commits a views' register delta and collects proofs
	CommitView(state.View, flow.StateCommitment) (flow.StateCommitment, []byte, *ledger.TrieUpdate, error)
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(
		context.Context,
		*entity.ExecutableBlock,
		state.View,
		*programs.BlockPrograms,
	) (
		*execution.ComputationResult,
		error,
	)
}

type blockComputer struct {
	vm                    VirtualMachine
	vmCtx                 fvm.Context
	metrics               module.ExecutionMetrics
	tracer                module.Tracer
	log                   zerolog.Logger
	systemChunkCtx        fvm.Context
	committer             ViewCommitter
	executionDataProvider *provider.Provider
}

func SystemChunkContext(vmCtx fvm.Context, logger zerolog.Logger) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithServiceEventCollectionEnabled(),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker()),
		fvm.WithEventCollectionSizeLimit(SystemChunkEventCollectionMaxSize),
		fvm.WithMemoryAndInteractionLimitsDisabled(),
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
	executionDataProvider *provider.Provider,
) (BlockComputer, error) {
	return &blockComputer{
		vm:                    vm,
		vmCtx:                 vmCtx,
		metrics:               metrics,
		tracer:                tracer,
		log:                   logger,
		systemChunkCtx:        SystemChunkContext(vmCtx, logger),
		committer:             committer,
		executionDataProvider: executionDataProvider,
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	program *programs.BlockPrograms,
) (*execution.ComputationResult, error) {

	span, _, isSampled := e.tracer.StartBlockSpan(ctx, block.ID(), trace.EXEComputeBlock)
	if isSampled {
		span.SetAttributes(attribute.Int("collection_counts", len(block.CompleteCollections)))
	}
	defer span.End()

	results, err := e.executeBlock(ctx, span, block, stateView, program)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	blockSpan otelTrace.Span,
	block *entity.ExecutableBlock,
	stateView state.View,
	programs *programs.BlockPrograms,
) (*execution.ComputationResult, error) {

	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	blockCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithBlockPrograms(programs))
	systemChunkCtx := fvm.NewContextFromParent(
		e.systemChunkCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithBlockPrograms(programs))
	collections := block.Collections()

	chunksSize := len(collections) + 1 // + 1 system chunk

	res := &execution.ComputationResult{
		ExecutableBlock:        block,
		Events:                 make([]flow.EventsList, chunksSize),
		ServiceEvents:          make(flow.EventsList, 0),
		TransactionResults:     make([]flow.TransactionResult, 0),
		TransactionResultIndex: make([]int, 0),
		StateCommitments:       make([]flow.StateCommitment, 0, chunksSize),
		Proofs:                 make([][]byte, 0, chunksSize),
		TrieUpdates:            make([]*ledger.TrieUpdate, 0, chunksSize),
		EventsHashes:           make([]flow.Identifier, 0, chunksSize),
	}

	var txIndex uint32
	var err error
	var wg sync.WaitGroup
	wg.Add(2) // block commiter and event hasher

	bc := blockCommitter{
		committer: e.committer,
		blockSpan: blockSpan,
		tracer:    e.tracer,
		metrics:   e.metrics,
		state:     *block.StartState,
		views:     make(chan state.View, chunksSize),
		res:       res,
	}
	defer bc.Close()

	eh := eventHasher{
		tracer:    e.tracer,
		data:      make(chan flow.EventsList, chunksSize),
		blockSpan: blockSpan,
		res:       res,
	}
	defer eh.Close()

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
		txIndex, err = e.executeCollection(blockSpan, collectionIndex, txIndex, blockCtx, colView, collection, res)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection at txIndex %v: %w", txIndex, err)
		}
		bc.Commit(colView)
		eh.Hash(res.Events[i])
		err = e.mergeView(stateView, colView, blockSpan, trace.EXEMergeCollectionView)
		if err != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err)
		}

		collectionIndex++
	}

	// executing system chunk
	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("executing system chunk")
	colView := stateView.NewChild()
	systemCol, err := e.executeSystemCollection(blockSpan, collectionIndex, txIndex, systemChunkCtx, colView, res)
	if err != nil {
		return nil, fmt.Errorf("failed to execute system chunk transaction: %w", err)
	}

	bc.Commit(colView)
	eh.Hash(res.Events[len(res.Events)-1])
	// close the views and wait for all views to be committed
	bc.Close()
	eh.Close()

	err = e.mergeView(stateView, colView, blockSpan, trace.EXEMergeCollectionView)
	if err != nil {
		return nil, fmt.Errorf("cannot merge view: %w", err)
	}

	wg.Wait()

	e.log.Debug().Hex("block_id", logging.Entity(block)).Msg("all views committed")

	executionData := generateExecutionData(res, collections, systemCol)

	executionDataID, err := e.executionDataProvider.Provide(ctx, block.Height(), executionData)
	if err != nil {
		return nil, fmt.Errorf("failed to provide execution data: %w", err)
	}

	res.ExecutionDataID = executionDataID

	return res, nil
}

func generateExecutionData(
	res *execution.ComputationResult,
	collections []*entity.CompleteCollection,
	systemCol *flow.Collection,
) *execution_data.BlockExecutionData {
	executionData := &execution_data.BlockExecutionData{
		BlockID:             res.ExecutableBlock.ID(),
		ChunkExecutionDatas: make([]*execution_data.ChunkExecutionData, 0, len(collections)+1),
	}

	for i, collection := range collections {
		col := collection.Collection()
		executionData.ChunkExecutionDatas = append(executionData.ChunkExecutionDatas, &execution_data.ChunkExecutionData{
			Collection: &col,
			Events:     res.Events[i],
			TrieUpdate: res.TrieUpdates[i],
		})
	}

	executionData.ChunkExecutionDatas = append(executionData.ChunkExecutionDatas, &execution_data.ChunkExecutionData{
		Collection: systemCol,
		Events:     res.Events[len(res.Events)-1],
		TrieUpdate: res.TrieUpdates[len(res.TrieUpdates)-1],
	})

	return executionData
}

func (e *blockComputer) executeSystemCollection(
	blockSpan otelTrace.Span,
	collectionIndex int,
	txIndex uint32,
	systemChunkCtx fvm.Context,
	collectionView state.View,
	res *execution.ComputationResult,
) (*flow.Collection, error) {
	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeSystemCollection)
	defer colSpan.End()

	startedAt := time.Now()
	tx, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf("could not get system chunk transaction: %w", err)
	}

	err = e.executeTransaction(tx, colSpan, collectionView, systemChunkCtx, collectionIndex, txIndex, res, true)

	if err != nil {
		return nil, err
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

	snapshot := collectionView.(*delta.View).Interactions()
	res.AddStateSnapshot(snapshot)
	res.UpdateTransactionResultIndex(1)

	compUsed, memUsed := res.ChunkComputationAndMemoryUsed(collectionIndex)
	eventCounts, eventSize := res.ChunkEventCountsAndSize(collectionIndex)
	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt),
		compUsed, memUsed,
		eventCounts, eventSize,
		snapshot.NumberOfRegistersTouched,
		snapshot.NumberOfBytesWrittenToRegisters,
		1,
	)

	return &flow.Collection{
		Transactions: []*flow.TransactionBody{tx},
	}, err
}

func (e *blockComputer) executeCollection(
	blockSpan otelTrace.Span,
	collectionIndex int,
	txIndex uint32,
	blockCtx fvm.Context,
	collectionView state.View,
	collection *entity.CompleteCollection,
	res *execution.ComputationResult,
) (uint32, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(blockCtx.BlockHeader)).
		Hex("collection_id", logging.Entity(collection.Guarantee)).
		Msg("executing collection")

	// call tracing
	startedAt := time.Now()
	colSpan := e.tracer.StartSpanFromParent(blockSpan, trace.EXEComputeCollection)
	defer func() {
		colSpan.SetAttributes(
			attribute.Int("collection.txCount", len(collection.Transactions)),
			attribute.String("collection.hash", collection.Guarantee.CollectionID.String()),
		)
		colSpan.End()
	}()

	txCtx := fvm.NewContextFromParent(blockCtx, fvm.WithMetricsReporter(e.metrics), fvm.WithTracer(e.tracer))
	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(txBody, colSpan, collectionView, txCtx, collectionIndex, txIndex, res, false)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}
	viewSnapshot := collectionView.(*delta.View).Interactions()
	res.AddStateSnapshot(viewSnapshot)
	res.UpdateTransactionResultIndex(len(collection.Transactions))
	e.log.Info().Str("collectionID", collection.Guarantee.CollectionID.String()).
		Str("referenceBlockID", collection.Guarantee.ReferenceBlockID.String()).
		Hex("blockID", logging.Entity(blockCtx.BlockHeader)).
		Int("numberOfTransactions", len(collection.Transactions)).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	compUsed, memUsed := res.ChunkComputationAndMemoryUsed(collectionIndex)
	eventCounts, eventSize := res.ChunkEventCountsAndSize(collectionIndex)
	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt),
		compUsed, memUsed,
		eventCounts, eventSize,
		viewSnapshot.NumberOfRegistersTouched,
		viewSnapshot.NumberOfBytesWrittenToRegisters,
		len(collection.Transactions),
	)
	return txIndex, nil
}

func (e *blockComputer) executeTransaction(
	txBody *flow.TransactionBody,
	colSpan otelTrace.Span,
	collectionView state.View,
	ctx fvm.Context,
	collectionIndex int,
	txIndex uint32,
	res *execution.ComputationResult,
	isSystemChunk bool,
) error {
	startedAt := time.Now()
	memAllocBefore := debug.GetHeapAllocsBytes()
	txID := txBody.ID()

	// we capture two spans one for tx-based view and one for the current context (block-based) view
	txSpan := e.tracer.StartSpanFromParent(colSpan, trace.EXEComputeTransaction)
	txSpan.SetAttributes(
		attribute.String("tx_id", txID.String()),
		attribute.Int64("tx_index", int64(txIndex)),
		attribute.Int("col_index", collectionIndex),
	)
	defer txSpan.End()

	var traceID string
	txInternalSpan, _, isSampled := e.tracer.StartTransactionSpan(context.Background(), txID, trace.EXERunTransaction)
	if isSampled {
		txInternalSpan.SetAttributes(attribute.String("tx_id", txID.String()))
		traceID = txInternalSpan.SpanContext().TraceID().String()
	}
	defer txInternalSpan.End()

	e.log.Info().
		Str("tx_id", txID.String()).
		Uint32("tx_index", txIndex).
		Str("block_id", res.ExecutableBlock.ID().String()).
		Uint64("height", res.ExecutableBlock.Block.Header.Height).
		Bool("system_chunk", isSystemChunk).
		Msg("executing transaction in fvm")

	tx := fvm.Transaction(txBody, txIndex)
	if isSampled {
		tx.SetTraceSpan(txInternalSpan)
	}

	txView := collectionView.NewChild()
	childCtx := fvm.NewContextFromParent(ctx,
		fvm.WithLogger(ctx.Logger.With().
			Str("tx_id", txID.String()).
			Uint32("tx_index", txIndex).
			Str("block_id", res.ExecutableBlock.ID().String()).
			Uint64("height", res.ExecutableBlock.Block.Header.Height).
			Bool("system_chunk", isSystemChunk).
			Logger()),
	)
	err := e.vm.Run(childCtx, tx, txView)
	if err != nil {
		return fmt.Errorf("failed to execute transaction %v for block %v at height %v: %w",
			txID.String(),
			res.ExecutableBlock.ID(),
			res.ExecutableBlock.Block.Header.Height,
			err)
	}

	txResult := flow.TransactionResult{
		TransactionID:   tx.ID,
		ComputationUsed: tx.ComputationUsed,
		MemoryUsed:      tx.MemoryEstimate,
	}

	if tx.Err != nil {
		txResult.ErrorMessage = tx.Err.Error()
	}

	postProcessSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEPostProcessTransaction)
	defer postProcessSpan.End()

	// always merge the view, fvm take cares of reverting changes
	// of failed transaction invocation

	err = e.mergeView(collectionView, txView, postProcessSpan, trace.EXEMergeTransactionView)
	if err != nil {
		return fmt.Errorf("merging tx view to collection view failed for tx %v: %w",
			txID.String(), err)
	}

	res.AddEvents(collectionIndex, tx.Events)
	res.AddServiceEvents(tx.ServiceEvents)
	res.AddTransactionResult(&txResult)

	memAllocAfter := debug.GetHeapAllocsBytes()

	lg := e.log.With().
		Hex("tx_id", txResult.TransactionID[:]).
		Str("block_id", res.ExecutableBlock.ID().String()).
		Str("traceID", traceID).
		Uint64("computation_used", txResult.ComputationUsed).
		Uint64("memory_used", tx.MemoryEstimate).
		Uint64("memAlloc", memAllocAfter-memAllocBefore).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Logger()

	if tx.Err != nil {
		lg.Info().
			Str("error_message", txResult.ErrorMessage).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction executed failed")
	} else {
		lg.Info().Msg("transaction executed successfully")
	}

	e.metrics.ExecutionTransactionExecuted(
		time.Since(startedAt),
		tx.ComputationUsed,
		tx.MemoryEstimate,
		memAllocAfter-memAllocBefore,
		len(tx.Events),
		flow.EventsList(tx.Events).ByteSize(),
		tx.Err != nil,
	)
	return nil
}

func (e *blockComputer) mergeView(
	parent, child state.View,
	parentSpan otelTrace.Span,
	mergeSpanName trace.SpanName) error {

	mergeSpan := e.tracer.StartSpanFromParent(parentSpan, mergeSpanName)
	defer mergeSpan.End()

	return parent.MergeView(child)
}

type blockCommitter struct {
	tracer    module.Tracer
	committer ViewCommitter
	state     flow.StateCommitment
	views     chan state.View
	closeOnce sync.Once
	blockSpan otelTrace.Span
	metrics   module.ExecutionMetrics

	res *execution.ComputationResult
}

func (bc *blockCommitter) Run() {
	for view := range bc.views {
		span := bc.tracer.StartSpanFromParent(bc.blockSpan, trace.EXECommitDelta)
		stateCommit, proof, trieUpdate, err := bc.committer.CommitView(view, bc.state)
		if err != nil {
			panic(err)
		}

		bc.res.StateCommitments = append(bc.res.StateCommitments, stateCommit)
		bc.res.Proofs = append(bc.res.Proofs, proof)
		bc.res.TrieUpdates = append(bc.res.TrieUpdates, trieUpdate)

		bc.state = stateCommit
		span.End()
	}
}

func (bc *blockCommitter) Commit(view state.View) {
	bc.views <- view
}

func (bc *blockCommitter) Close() {
	bc.closeOnce.Do(func() { close(bc.views) })
}

type eventHasher struct {
	tracer    module.Tracer
	data      chan flow.EventsList
	closeOnce sync.Once
	blockSpan otelTrace.Span

	res *execution.ComputationResult
}

func (eh *eventHasher) Run() {
	for data := range eh.data {
		span := eh.tracer.StartSpanFromParent(eh.blockSpan, trace.EXEHashEvents)
		rootHash, err := flow.EventsMerkleRootHash(data)
		if err != nil {
			panic(err)
		}

		eh.res.EventsHashes = append(eh.res.EventsHashes, rootHash)

		span.End()
	}
}

func (eh *eventHasher) Hash(events flow.EventsList) {
	eh.data <- events
}

func (eh *eventHasher) Close() {
	eh.closeOnce.Do(func() { close(eh.data) })
}
