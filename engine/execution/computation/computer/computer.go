package computer

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
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
	blockId string

	collectionIndex int

	*entity.CompleteCollection

	ctx fvm.Context

	isSystemCollection bool
}

func (collection collectionItem) Transactions(
	startTxIndex uint32,
) []transaction {
	txns := make(
		[]transaction,
		0,
		len(collection.CompleteCollection.Transactions))

	logger := collection.ctx.Logger.With().
		Str("block_id", collection.blockId).
		Uint64("height", collection.ctx.BlockHeader.Height).
		Bool("system_chunk", collection.isSystemCollection).
		Bool("system_transaction", collection.isSystemCollection).
		Logger()

	for idx, txBody := range collection.CompleteCollection.Transactions {
		txIndex := startTxIndex + uint32(idx)
		txns = append(
			txns,
			transaction{
				blockIdStr:          collection.blockId,
				collectionIndex:     collection.collectionIndex,
				txIndex:             txIndex,
				isSystemTransaction: collection.isSystemCollection,
				ctx: fvm.NewContextFromParent(
					collection.ctx,
					fvm.WithLogger(
						logger.With().
							Str("tx_id", txBody.ID().String()).
							Uint32("tx_index", txIndex).
							Logger())),
				TransactionBody: txBody,
			})
	}

	return txns
}

type transaction struct {
	blockIdStr string

	collectionIndex int
	txIndex         uint32

	isSystemTransaction bool

	ctx fvm.Context
	*flow.TransactionBody
}

// VirtualMachine runs procedures
type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View) error
	GetAccount(fvm.Context, flow.Address, state.View) (*flow.Account, error)
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(
		context.Context,
		*entity.ExecutableBlock,
		state.View,
		*derived.DerivedBlockData,
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
	executionDataProvider provider.Provider
	signer                module.Local
	spockHasher           hash.Hasher
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
	signer module.Local,
	executionDataProvider provider.Provider,
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
		signer:                signer,
		spockHasher:           utils.NewSPOCKHasher(),
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	stateView state.View,
	derivedBlockData *derived.DerivedBlockData,
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
	derivedBlockData *derived.DerivedBlockData,
) (
	[]collectionItem,
	error,
) {
	rawCollections := block.Collections()
	collections := make([]collectionItem, 0, len(rawCollections)+1)

	blockIdStr := block.ID().String()

	blockCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithDerivedBlockData(derivedBlockData))

	for idx, collection := range rawCollections {
		collections = append(
			collections,
			collectionItem{
				blockId:            blockIdStr,
				collectionIndex:    idx,
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
			blockId:         blockIdStr,
			collectionIndex: len(collections),
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
	derivedBlockData *derived.DerivedBlockData,
) (*execution.ComputationResult, error) {

	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	collections, err := e.getCollections(block, derivedBlockData)
	if err != nil {
		return nil, err
	}

	collector := newResultCollector(
		e.tracer,
		blockSpan,
		e.metrics,
		e.committer,
		e.signer,
		e.spockHasher,
		block,
		len(collections))
	defer collector.Stop()

	var txIndex uint32
	for _, collection := range collections {
		colView := stateView.NewChild()
		txIndex, err = e.executeCollection(
			blockSpan,
			txIndex,
			colView,
			collection,
			collector)
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
		err = e.mergeView(
			stateView,
			colView,
			blockSpan,
			trace.EXEMergeCollectionView)
		if err != nil {
			return nil, fmt.Errorf("cannot merge view: %w", err)
		}
	}

	res, err := collector.Finalize()
	if err != nil {
		return nil, fmt.Errorf("cannot finalize computation result: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Msg("all views committed")

	executionDataID, executionDataRoot, err := e.executionDataProvider.Provide(
		ctx,
		block.Height(),
		generateExecutionData(res, collections))
	if err != nil {
		return nil, fmt.Errorf("failed to provide execution data: %w", err)
	}

	res.ExecutionDataRoot = executionDataRoot
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
	startTxIndex uint32,
	collectionView state.View,
	collection collectionItem,
	collector *resultCollector,
) (uint32, error) {

	// call tracing
	startedAt := time.Now()

	txns := collection.Transactions(startTxIndex)

	colSpanType := trace.EXEComputeSystemCollection
	collectionId := ""
	referenceBlockId := ""
	if !collection.isSystemCollection {
		colSpanType = trace.EXEComputeCollection
		collectionId = collection.Guarantee.CollectionID.String()
		referenceBlockId = collection.Guarantee.ReferenceBlockID.String()
	}

	colSpan := e.tracer.StartSpanFromParent(blockSpan, colSpanType)
	defer colSpan.End()

	colSpan.SetAttributes(
		attribute.Int("collection.txCount", len(txns)),
		attribute.String("collection.hash", collectionId))

	logger := e.log.With().
		Str("block_id", collection.blockId).
		Str("collection_id", collectionId).
		Str("reference_block_id", referenceBlockId).
		Int("number_of_transactions", len(txns)).
		Bool("system_collection", collection.isSystemCollection).
		Logger()
	logger.Debug().Msg("executing collection")

	for _, txn := range txns {
		err := e.executeTransaction(colSpan, txn, collectionView, collector)
		if err != nil {
			return txn.txIndex, err
		}
	}

	logger.Info().
		Int64("time_spent_in_ms", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	stats := collector.CommitCollection(
		collection,
		collectionView)

	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt), stats)
	return startTxIndex + uint32(len(txns)), nil
}

func (e *blockComputer) executeTransaction(
	parentSpan otelTrace.Span,
	txn transaction,
	collectionView state.View,
	collector *resultCollector,
) error {
	startedAt := time.Now()
	memAllocBefore := debug.GetHeapAllocsBytes()
	txID := txn.ID()

	// we capture two spans one for tx-based view and one for the current context (block-based) view
	txSpan := e.tracer.StartSpanFromParent(parentSpan, trace.EXEComputeTransaction)
	txSpan.SetAttributes(
		attribute.String("tx_id", txID.String()),
		attribute.Int64("tx_index", int64(txn.txIndex)),
		attribute.Int("col_index", txn.collectionIndex),
	)
	defer txSpan.End()

	var traceID string
	txInternalSpan, _, isSampled := e.tracer.StartTransactionSpan(context.Background(), txID, trace.EXERunTransaction)
	if isSampled {
		txInternalSpan.SetAttributes(attribute.String("tx_id", txID.String()))
		traceID = txInternalSpan.SpanContext().TraceID().String()
	}
	defer txInternalSpan.End()

	logger := e.log.With().
		Str("tx_id", txID.String()).
		Uint32("tx_index", txn.txIndex).
		Str("block_id", txn.blockIdStr).
		Str("trace_id", traceID).
		Uint64("height", txn.ctx.BlockHeader.Height).
		Bool("system_chunk", txn.isSystemTransaction).
		Bool("system_transaction", txn.isSystemTransaction).
		Logger()
	logger.Info().Msg("executing transaction in fvm")

	proc := fvm.Transaction(txn.TransactionBody, txn.txIndex)
	if isSampled {
		proc.SetTraceSpan(txInternalSpan)
	}

	txView := collectionView.NewChild()
	err := e.vm.Run(txn.ctx, proc, txView)
	if err != nil {
		return fmt.Errorf("failed to execute transaction %v for block %s at height %v: %w",
			txID.String(),
			txn.blockIdStr,
			txn.ctx.BlockHeader.Height,
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

	collector.AddTransactionResult(txn.collectionIndex, proc)

	memAllocAfter := debug.GetHeapAllocsBytes()

	logger = logger.With().
		Uint64("computation_used", proc.ComputationUsed).
		Uint64("memory_used", proc.MemoryEstimate).
		Uint64("mem_alloc", memAllocAfter-memAllocBefore).
		Int64("time_spent_in_ms", time.Since(startedAt).Milliseconds()).
		Logger()

	if proc.Err != nil {
		logger = logger.With().
			Str("error_message", proc.Err.Error()).
			Uint16("error_code", uint16(proc.Err.Code())).
			Logger()
		logger.Info().Msg("transaction execution failed")

		if txn.isSystemTransaction {
			// This log is used as the data source for an alert on grafana.
			// The system_chunk_error field must not be changed without adding
			// the corresponding changes in grafana.
			// https://github.com/dapperlabs/flow-internal/issues/1546
			logger.Error().
				Bool("system_chunk_error", true).
				Bool("system_transaction_error", true).
				Bool("critical_error", true).
				Msg("error executing system chunk transaction")
		}
	} else {
		logger.Info().Msg("transaction executed successfully")
	}

	e.metrics.ExecutionTransactionExecuted(
		time.Since(startedAt),
		proc.ComputationUsed,
		proc.MemoryEstimate,
		memAllocAfter-memAllocBefore,
		len(proc.Events),
		flow.EventsList(proc.Events).ByteSize(),
		proc.Err != nil,
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
