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

	*entity.CompleteCollection

	ctx fvm.Context

	isSystemCollection bool
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
	executionDataProvider *provider.Provider
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

	for _, collection := range rawCollections {
		collections = append(
			collections,
			collectionItem{
				blockId:            blockIdStr,
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
			blockId: blockIdStr,
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
	for collectionIndex, collection := range collections {
		colView := stateView.NewChild()
		txIndex, err = e.executeCollection(
			blockSpan,
			collectionIndex,
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
	collector *resultCollector,
) (uint32, error) {

	// call tracing
	startedAt := time.Now()

	var colSpan otelTrace.Span
	if collection.isSystemCollection {
		colSpan = e.tracer.StartSpanFromParent(
			blockSpan,
			trace.EXEComputeSystemCollection)

		e.log.Debug().
			Str("block_id", collection.blockId).
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
			Str("block_id", collection.blockId).
			Hex("collection_id", logging.Entity(collection.Guarantee)).
			Msg("executing collection")
	}

	defer colSpan.End()

	for _, txBody := range collection.Transactions {
		err := e.executeTransaction(collection.blockId, txBody, colSpan, collectionView, collection.ctx, collectionIndex, txIndex, collector, collection.isSystemCollection)
		txIndex++
		if err != nil {
			return txIndex, err
		}
	}

	if !collection.isSystemCollection {
		e.log.Info().
			Str("collectionID", collection.Guarantee.CollectionID.String()).
			Str("referenceBlockID",
				collection.Guarantee.ReferenceBlockID.String()).
			Str("block_id", collection.blockId).
			Int("numberOfTransactions", len(collection.Transactions)).
			Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
			Msg("collection executed")
	}

	stats := collector.CommitCollection(
		collectionIndex,
		collection,
		collectionView)

	e.metrics.ExecutionCollectionExecuted(time.Since(startedAt), stats)
	return txIndex, nil
}

func (e *blockComputer) executeTransaction(
	blockIdStr string,
	txBody *flow.TransactionBody,
	colSpan otelTrace.Span,
	collectionView state.View,
	ctx fvm.Context,
	collectionIndex int,
	txIndex uint32,
	collector *resultCollector,
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
		Str("block_id", blockIdStr).
		Uint64("height", ctx.BlockHeader.Height).
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
			Str("block_id", blockIdStr).
			Uint64("height", ctx.BlockHeader.Height).
			Bool("system_chunk", isSystemTransaction).
			Bool("system_transaction", isSystemTransaction).
			Logger()),
	)
	err := e.vm.Run(childCtx, tx, txView)
	if err != nil {
		return fmt.Errorf("failed to execute transaction %v for block %s at height %v: %w",
			txID.String(),
			blockIdStr,
			ctx.BlockHeader.Height,
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

	collector.AddTransactionResult(collectionIndex, tx)

	memAllocAfter := debug.GetHeapAllocsBytes()

	lg := e.log.With().
		Str("tx_id", txID.String()).
		Str("block_id", blockIdStr).
		Str("traceID", traceID).
		Uint64("computation_used", tx.ComputationUsed).
		Uint64("memory_used", tx.MemoryEstimate).
		Uint64("memAlloc", memAllocAfter-memAllocBefore).
		Int64("timeSpentInMS", time.Since(startedAt).Milliseconds()).
		Logger()

	if tx.Err != nil {
		errMsg := tx.Err.Error()

		lg.Info().
			Str("error_message", errMsg).
			Uint16("error_code", uint16(tx.Err.Code())).
			Msg("transaction execution failed")

		if isSystemTransaction {
			// This log is used as the data source for an alert on grafana.
			// The system_chunk_error field must not be changed without adding
			// the corresponding changes in grafana.
			// https://github.com/dapperlabs/flow-internal/issues/1546
			e.log.Error().
				Str("error_message", errMsg).
				Hex("block_id", logging.Entity(ctx.BlockHeader)).
				Bool("system_chunk_error", true).
				Bool("system_transaction_error", true).
				Bool("critical_error", true).
				Msg("error executing system chunk transaction")
		}
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
