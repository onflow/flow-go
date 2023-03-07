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
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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
	blockId    flow.Identifier
	blockIdStr string

	collectionIndex int

	*entity.CompleteCollection

	isSystemCollection bool

	transactions []transaction
}

func newTransactions(
	blockId flow.Identifier,
	blockIdStr string,
	collectionIndex int,
	collectionCtx fvm.Context,
	isSystemCollection bool,
	startTxnIndex int,
	txnBodies []*flow.TransactionBody,
) []transaction {
	txns := make([]transaction, 0, len(txnBodies))

	logger := collectionCtx.Logger.With().
		Str("block_id", blockIdStr).
		Uint64("height", collectionCtx.BlockHeader.Height).
		Bool("system_chunk", isSystemCollection).
		Bool("system_transaction", isSystemCollection).
		Logger()

	for idx, txnBody := range txnBodies {
		txnId := txnBody.ID()
		txnIdStr := txnId.String()
		txnIndex := uint32(startTxnIndex + idx)
		txns = append(
			txns,
			transaction{
				blockId:             blockId,
				blockIdStr:          blockIdStr,
				txnId:               txnId,
				txnIdStr:            txnIdStr,
				collectionIndex:     collectionIndex,
				txnIndex:            txnIndex,
				isSystemTransaction: isSystemCollection,
				ctx: fvm.NewContextFromParent(
					collectionCtx,
					fvm.WithLogger(
						logger.With().
							Str("tx_id", txnIdStr).
							Uint32("tx_index", txnIndex).
							Logger())),
				TransactionBody: txnBody,
			})
	}

	return txns
}

type transaction struct {
	blockId    flow.Identifier
	blockIdStr string

	txnId    flow.Identifier
	txnIdStr string

	collectionIndex int
	txnIndex        uint32

	isSystemTransaction bool

	ctx fvm.Context
	*flow.TransactionBody
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(
		ctx context.Context,
		parentBlockExecutionResultID flow.Identifier,
		block *entity.ExecutableBlock,
		snapshot state.StorageSnapshot,
		derivedBlockData *derived.DerivedBlockData,
	) (
		*execution.ComputationResult,
		error,
	)
}

type blockComputer struct {
	vm                    fvm.VM
	vmCtx                 fvm.Context
	metrics               module.ExecutionMetrics
	tracer                module.Tracer
	log                   zerolog.Logger
	systemChunkCtx        fvm.Context
	committer             ViewCommitter
	executionDataProvider *provider.Provider
	signer                module.Local
	spockHasher           hash.Hasher
	receiptHasher         hash.Hasher
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
	vm fvm.VM,
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
		receiptHasher:         utils.NewExecutionReceiptHasher(),
	}, nil
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	ctx context.Context,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	snapshot state.StorageSnapshot,
	derivedBlockData *derived.DerivedBlockData,
) (
	*execution.ComputationResult,
	error,
) {
	results, err := e.executeBlock(
		ctx,
		parentBlockExecutionResultID,
		block,
		snapshot,
		derivedBlockData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	return results, nil
}

func (e *blockComputer) getRootSpanAndCollections(
	block *entity.ExecutableBlock,
	derivedBlockData *derived.DerivedBlockData,
) (
	otelTrace.Span,
	[]collectionItem,
	error,
) {
	rawCollections := block.Collections()
	collections := make([]collectionItem, 0, len(rawCollections)+1)

	blockId := block.ID()
	blockIdStr := blockId.String()

	blockCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithDerivedBlockData(derivedBlockData))

	startTxnIndex := 0
	for idx, collection := range rawCollections {
		collections = append(
			collections,
			collectionItem{
				blockId:            blockId,
				blockIdStr:         blockIdStr,
				collectionIndex:    idx,
				CompleteCollection: collection,
				isSystemCollection: false,

				transactions: newTransactions(
					blockId,
					blockIdStr,
					idx,
					blockCtx,
					false,
					startTxnIndex,
					collection.Transactions),
			})
		startTxnIndex += len(collection.Transactions)
	}

	systemTxn, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return trace.NoopSpan, nil, fmt.Errorf(
			"could not get system chunk transaction: %w",
			err)
	}

	systemCtx := fvm.NewContextFromParent(
		e.systemChunkCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithDerivedBlockData(derivedBlockData))
	systemTransactions := []*flow.TransactionBody{systemTxn}

	collections = append(
		collections,
		collectionItem{
			blockId:         blockId,
			blockIdStr:      blockIdStr,
			collectionIndex: len(collections),
			CompleteCollection: &entity.CompleteCollection{
				Transactions: systemTransactions,
			},
			isSystemCollection: true,

			transactions: newTransactions(
				blockId,
				blockIdStr,
				len(rawCollections),
				systemCtx,
				true,
				startTxnIndex,
				systemTransactions),
		})

	return e.tracer.BlockRootSpan(blockId), collections, nil
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	snapshot state.StorageSnapshot,
	derivedBlockData *derived.DerivedBlockData,
) (
	*execution.ComputationResult,
	error,
) {
	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	rootSpan, collections, err := e.getRootSpanAndCollections(
		block,
		derivedBlockData)
	if err != nil {
		return nil, err
	}

	blockSpan := e.tracer.StartSpanFromParent(rootSpan, trace.EXEComputeBlock)
	blockSpan.SetAttributes(
		attribute.String("block_id", block.ID().String()),
		attribute.Int("collection_counts", len(block.CompleteCollections)))
	defer blockSpan.End()

	collector := newResultCollector(
		e.tracer,
		blockSpan,
		e.metrics,
		e.committer,
		e.signer,
		e.executionDataProvider,
		e.spockHasher,
		e.receiptHasher,
		parentBlockExecutionResultID,
		block,
		len(collections))
	defer collector.Stop()

	stateView := delta.NewDeltaView(snapshot)

	var txnIndex uint32
	for _, collection := range collections {
		colView := stateView.NewChild()
		txnIndex, err = e.executeCollection(
			blockSpan,
			txnIndex,
			colView,
			collection,
			collector)
		if err != nil {
			collectionPrefix := ""
			if collection.isSystemCollection {
				collectionPrefix = "system "
			}

			return nil, fmt.Errorf(
				"failed to execute %scollection at txnIndex %v: %w",
				collectionPrefix,
				txnIndex,
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

	res, err := collector.Finalize(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot finalize computation result: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(block)).
		Msg("all views committed")

	e.metrics.ExecutionBlockCachedPrograms(derivedBlockData.CachedPrograms())

	return res, nil
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

	txns := collection.transactions

	collectionId := ""
	referenceBlockId := ""
	if !collection.isSystemCollection {
		collectionId = collection.Guarantee.CollectionID.String()
		referenceBlockId = collection.Guarantee.ReferenceBlockID.String()
	}

	logger := e.log.With().
		Str("block_id", collection.blockIdStr).
		Str("collection_id", collectionId).
		Str("reference_block_id", referenceBlockId).
		Int("number_of_transactions", len(txns)).
		Bool("system_collection", collection.isSystemCollection).
		Logger()
	logger.Debug().Msg("executing collection")

	for _, txn := range txns {
		err := e.executeTransaction(blockSpan, txn, collectionView, collector)
		if err != nil {
			return txn.txnIndex, err
		}
	}

	logger.Info().
		Int64("time_spent_in_ms", time.Since(startedAt).Milliseconds()).
		Msg("collection executed")

	collector.CommitCollection(
		collection,
		startedAt,
		collectionView)

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

	txSpan := e.tracer.StartSampledSpanFromParent(
		parentSpan,
		txn.txnId,
		trace.EXEComputeTransaction)
	txSpan.SetAttributes(
		attribute.String("tx_id", txn.txnIdStr),
		attribute.Int64("tx_index", int64(txn.txnIndex)),
		attribute.Int("col_index", txn.collectionIndex),
	)
	defer txSpan.End()

	logger := e.log.With().
		Str("tx_id", txn.txnIdStr).
		Uint32("tx_index", txn.txnIndex).
		Str("block_id", txn.blockIdStr).
		Uint64("height", txn.ctx.BlockHeader.Height).
		Bool("system_chunk", txn.isSystemTransaction).
		Bool("system_transaction", txn.isSystemTransaction).
		Logger()
	logger.Info().Msg("executing transaction in fvm")

	proc := fvm.NewTransaction(txn.txnId, txn.txnIndex, txn.TransactionBody)

	txn.ctx = fvm.NewContextFromParent(txn.ctx, fvm.WithSpan(txSpan))

	txView := collectionView.NewChild()
	err := e.vm.Run(txn.ctx, proc, txView)
	if err != nil {
		return fmt.Errorf("failed to execute transaction %v for block %s at height %v: %w",
			txn.txnIdStr,
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
		return fmt.Errorf(
			"merging tx view to collection view failed for tx %v: %w",
			txn.txnIdStr,
			err)
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

	return parent.Merge(child)
}
