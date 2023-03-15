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
	"github.com/onflow/flow-go/engine/execution/computation/result"
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

type collectionInfo struct {
	blockId    flow.Identifier
	blockIdStr string

	collectionIndex int
	*entity.CompleteCollection

	isSystemTransaction bool
}

func newTransactions(
	collection collectionInfo,
	collectionCtx fvm.Context,
	startTxnIndex int,
) []transaction {
	txns := make([]transaction, 0, len(collection.Transactions))

	logger := collectionCtx.Logger.With().
		Str("block_id", collection.blockIdStr).
		Uint64("height", collectionCtx.BlockHeader.Height).
		Bool("system_chunk", collection.isSystemTransaction).
		Bool("system_transaction", collection.isSystemTransaction).
		Logger()

	for idx, txnBody := range collection.Transactions {
		txnId := txnBody.ID()
		txnIdStr := txnId.String()
		txnIndex := uint32(startTxnIndex + idx)
		txns = append(
			txns,
			transaction{
				collectionInfo: collection,
				txnId:          txnId,
				txnIdStr:       txnIdStr,
				txnIndex:       txnIndex,
				ctx: fvm.NewContextFromParent(
					collectionCtx,
					fvm.WithLogger(
						logger.With().
							Str("tx_id", txnIdStr).
							Uint32("tx_index", txnIndex).
							Logger())),
				TransactionProcedure: fvm.NewTransaction(
					txnId,
					txnIndex,
					txnBody),
			})
	}

	if len(txns) > 0 {
		txns[len(txns)-1].lastTransactionInCollection = true
	}

	return txns
}

type transaction struct {
	collectionInfo

	txnId    flow.Identifier
	txnIdStr string

	txnIndex uint32

	lastTransactionInCollection bool

	ctx fvm.Context
	*fvm.TransactionProcedure
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
	colResCons            []result.ExecutedCollectionConsumer
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
	colResCons []result.ExecutedCollectionConsumer,
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
		colResCons:            colResCons,
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

func (e *blockComputer) getRootSpanAndTransactions(
	block *entity.ExecutableBlock,
	derivedBlockData *derived.DerivedBlockData,
) (
	otelTrace.Span,
	[]transaction,
	error,
) {
	rawCollections := block.Collections()
	var transactions []transaction

	blockId := block.ID()
	blockIdStr := blockId.String()

	blockCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithDerivedBlockData(derivedBlockData))

	startTxnIndex := 0
	for idx, collection := range rawCollections {
		transactions = append(
			transactions,
			newTransactions(
				collectionInfo{
					blockId:             blockId,
					blockIdStr:          blockIdStr,
					collectionIndex:     idx,
					CompleteCollection:  collection,
					isSystemTransaction: false,
				},
				blockCtx,
				startTxnIndex)...)
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
	systemCollection := &entity.CompleteCollection{
		Transactions: []*flow.TransactionBody{systemTxn},
	}

	transactions = append(
		transactions,
		newTransactions(
			collectionInfo{
				blockId:             blockId,
				blockIdStr:          blockIdStr,
				collectionIndex:     len(rawCollections),
				CompleteCollection:  systemCollection,
				isSystemTransaction: true,
			},
			systemCtx,
			startTxnIndex)...)

	return e.tracer.BlockRootSpan(blockId), transactions, nil
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

	rootSpan, transactions, err := e.getRootSpanAndTransactions(
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
		len(transactions),
		e.colResCons)
	defer collector.Stop()

	stateView := delta.NewDeltaView(snapshot)
	for _, txn := range transactions {
		err := e.executeTransaction(blockSpan, txn, stateView, collector)
		if err != nil {
			prefix := ""
			if txn.isSystemTransaction {
				prefix = "system "
			}

			return nil, fmt.Errorf(
				"failed to execute %stransaction at txnIndex %v: %w",
				prefix,
				txn.txnIndex,
				err)
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

func (e *blockComputer) executeTransaction(
	parentSpan otelTrace.Span,
	txn transaction,
	stateView state.View,
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

	txn.ctx = fvm.NewContextFromParent(txn.ctx, fvm.WithSpan(txSpan))

	txView := stateView.NewChild()
	err := e.vm.Run(txn.ctx, txn.TransactionProcedure, txView)
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

	err = e.mergeView(stateView, txView, postProcessSpan, trace.EXEMergeTransactionView)
	if err != nil {
		return fmt.Errorf(
			"merging tx view to collection view failed for tx %v: %w",
			txn.txnIdStr,
			err)
	}

	collector.AddTransactionResult(txn, txView)

	memAllocAfter := debug.GetHeapAllocsBytes()

	logger = logger.With().
		Uint64("computation_used", txn.ComputationUsed).
		Uint64("memory_used", txn.MemoryEstimate).
		Uint64("mem_alloc", memAllocAfter-memAllocBefore).
		Int64("time_spent_in_ms", time.Since(startedAt).Milliseconds()).
		Logger()

	if txn.Err != nil {
		logger = logger.With().
			Str("error_message", txn.Err.Error()).
			Uint16("error_code", uint16(txn.Err.Code())).
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
		txn.ComputationUsed,
		txn.MemoryEstimate,
		memAllocAfter-memAllocBefore,
		len(txn.Events),
		flow.EventsList(txn.Events).ByteSize(),
		txn.Err != nil,
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
