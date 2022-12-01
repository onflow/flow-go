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

type collectionItem struct {
	*entity.CompleteCollection

	ctx fvm.Context

	isSystemCollection bool
}

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
		*programs.DerivedBlockData,
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
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithServiceEventCollectionEnabled(),
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
	systemChunkCtx := SystemChunkContext(vmCtx, logger)
	vmCtx = fvm.NewContextFromParent(
		vmCtx,
		fvm.WithMetricsReporter(metrics),
		fvm.WithTracer(tracer))
	return &blockComputer{
		vm:                    vm,
		vmCtx:                 vmCtx,
		metrics:               metrics,
		tracer:                tracer,
		log:                   logger,
		systemChunkCtx:        systemChunkCtx,
		committer:             committer,
		executionDataProvider: executionDataProvider,
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	derivedBlockData *programs.DerivedBlockData,
) (*execution.ComputationResult, error) {

	span, _, isSampled := e.tracer.StartBlockSpan(ctx, block.ID(), trace.EXEComputeBlock)
	if isSampled {
		span.SetAttributes(attribute.Int("collection_counts", len(block.CompleteCollections)))
	}
	defer span.End()

	results, err := e.executeBlock(ctx, span, block, stateView, derivedBlockData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) getCollections(
	block *entity.ExecutableBlock,
	derivedBlockData *programs.DerivedBlockData,
) (
	[]collectionItem,
	error,
) {
	rawCollections := block.Collections()
	collections := make([]collectionItem, 0, len(rawCollections)+1)

	blockCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithDerivedBlockData(derivedBlockData))

	for _, collection := range rawCollections {
		collections = append(
			collections,
			collectionItem{
				CompleteCollection: collection,
				ctx:                blockCtx,
				isSystemCollection: false,
			})
	}

	systemTxn, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get system chunk transaction: %w",
			err)
	}

	collections = append(
		collections,
		collectionItem{
			CompleteCollection: &entity.CompleteCollection{
				Transactions: []*flow.TransactionBody{systemTxn},
			},
			ctx: fvm.NewContextFromParent(
				e.systemChunkCtx,
				fvm.WithBlockHeader(block.Block.Header),
				fvm.WithDerivedBlockData(derivedBlockData)),
			isSystemCollection: true,
		})

	return collections, nil
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	blockSpan otelTrace.Span,
	block *entity.ExecutableBlock,
	stateView state.View,
	derivedBlockData *programs.DerivedBlockData,
) (*execution.ComputationResult, error) {

	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	collections, err := e.getCollections(block, derivedBlockData)
	if err != nil {
		return nil, err
	}

	res := execution.NewEmptyComputationResult(block)

	var txIndex uint32
	var wg sync.WaitGroup
	wg.Add(2) // block commiter and event hasher

	chunksSize := len(collections) + 1 // + 1 system chunk
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

	for collectionIndex, collection := range collections {
		colView := stateView.NewChild()
		txIndex, err = e.executeCollection(
			blockSpan,
			collectionIndex,
			txIndex,
			colView,
			collection,
			res)
		if err != nil {
			collectionPrefix := ""
			if collection.isSystemCollection {
				collectionPrefix = "system "
			}

			return nil, fmt.Errorf(
				"failed to execute %scollection at txIndex %v: %w",
				collectionPrefix,
				txIndex,
				err)
		}
		bc.Commit(colView)
		eh.Hash(res.Events[collectionIndex])
		err = e.mergeView(
			stateView,
			colView,
			blockSpan,
			trace.EXEMergeCollectionView)
		if err != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err)
		}
	}

	// close the views and wait for all views to be committed
	bc.Close()
	eh.Close()
	wg.Wait()

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Msg("all views committed")

	executionDataID, err := e.executionDataProvider.Provide(
		ctx,
		block.Height(),
		generateExecutionData(res, collections))
	if err != nil {
		return nil, fmt.Errorf("failed to provide execution data: %w", err)
	}

	res.ExecutionDataID = executionDataID

	return res, nil
}

func generateExecutionData(
	res *execution.ComputationResult,
	collections []collectionItem,
) *execution_data.BlockExecutionData {
	executionData := &execution_data.BlockExecutionData{
		BlockID: res.ExecutableBlock.ID(),
		ChunkExecutionDatas: make(
			[]*execution_data.ChunkExecutionData,
			0,
			len(collections)),
	}

	for i, collection := range collections {
		col := collection.Collection()
		executionData.ChunkExecutionDatas = append(executionData.ChunkExecutionDatas, &execution_data.ChunkExecutionData{
			Collection: &col,
			Events:     res.Events[i],
			TrieUpdate: res.TrieUpdates[i],
		})
	}

	return executionData
}

func (e *blockComputer) executeCollection(
	blockSpan otelTrace.Span,
	collectionIndex int,
	txIndex uint32,
	collectionView state.View,
	collection collectionItem,
	res *execution.ComputationResult,
) (uint32, error) {

	// call tracing
	startedAt := time.Now()

	var colSpan otelTrace.Span
	if collection.isSystemCollection {
		colSpan = e.tracer.StartSpanFromParent(
			blockSpan,
			trace.EXEComputeSystemCollection)

		e.log.Debug().
			Hex("block_id", logging.Entity(collection.ctx.BlockHeader)).
			Msg("executing system collection")
	} else {
		colSpan = e.tracer.StartSpanFromParent(
			blockSpan,
			trace.EXEComputeCollection)

		colSpan.SetAttributes(
			attribute.Int("collection.txCount", len(collection.Transactions)),
			attribute.String(
				"collection.hash",
				collection.Guarantee.CollectionID.String()))

		e.log.Debug().
			Hex("block_id", logging.Entity(collection.ctx.BlockHeader)).
			Hex("collection_id", logging.Entity(collection.Guarantee)).
			Msg("executing collection")
	}

	defer colSpan.End()

	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(txBody, colSpan, collectionView, collection.ctx, collectionIndex, txIndex, res, false)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}

	if collection.isSystemCollection {
		systemChunkTxResult := res.TransactionResults[len(res.TransactionResults)-1]
		if systemChunkTxResult.ErrorMessage != "" {
			// This log is used as the data source for an alert on grafana.
			// The system_chunk_error field must not be changed without adding
			// the corresponding changes in grafana.
			// https://github.com/dapperlabs/flow-internal/issues/1546
			e.log.Error().
				Str("error_message", systemChunkTxResult.ErrorMessage).
				Hex("block_id", logging.Entity(collection.ctx.BlockHeader)).
				Bool("system_chunk_error", true).
				Bool("system_transaction_error", true).
				Bool("critical_error", true).
				Msg("error executing system chunk transaction")
		}
	} else {
		e.log.Info().
			Str("collectionID", collection.Guarantee.CollectionID.String()).
			Str("referenceBlockID",
				collection.Guarantee.ReferenceBlockID.String()).
			Hex("blockID", logging.Entity(collection.ctx.BlockHeader)).
			Int("numberOfTransactions", len(collection.Transactions)).
			Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
			Msg("collection executed")
	}

	viewSnapshot := collectionView.(*delta.View).Interactions()
	res.AddStateSnapshot(viewSnapshot)
	res.UpdateTransactionResultIndex(len(collection.Transactions))

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
	isSystemTransaction bool,
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
		Bool("system_chunk", isSystemTransaction).
		Bool("system_transaction", isSystemTransaction).
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
			Bool("system_chunk", isSystemTransaction).
			Bool("system_transaction", isSystemTransaction).
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

	postProcessSpan := e.tracer.StartSpanFromParent(txSpan, trace.EXEPostProcessTransaction)
	defer postProcessSpan.End()

	// always merge the view, fvm take cares of reverting changes
	// of failed transaction invocation

	err = e.mergeView(collectionView, txView, postProcessSpan, trace.EXEMergeTransactionView)
	if err != nil {
		return fmt.Errorf("merging tx view to collection view failed for tx %v: %w",
			txID.String(), err)
	}

	res.AddTransactionResult(collectionIndex, tx)

	memAllocAfter := debug.GetHeapAllocsBytes()

	lg := e.log.With().
		Str("tx_id", txID.String()).
		Str("block_id", res.ExecutableBlock.ID().String()).
		Str("traceID", traceID).
		Uint64("computation_used", tx.ComputationUsed).
		Uint64("memory_used", tx.MemoryEstimate).
		Uint64("memAlloc", memAllocAfter-memAllocBefore).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Logger()

	if tx.Err != nil {
		lg.Info().
			Str("error_message", tx.Err.Error()).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction execution failed")
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
