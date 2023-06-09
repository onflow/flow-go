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
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
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

type transactionRequest struct {
	collectionInfo

	txnId    flow.Identifier
	txnIdStr string

	txnIndex uint32

	lastTransactionInCollection bool

	ctx fvm.Context
	*fvm.TransactionProcedure
}

func newTransactionRequest(
	collection collectionInfo,
	collectionCtx fvm.Context,
	collectionLogger zerolog.Logger,
	txnIndex uint32,
	txnBody *flow.TransactionBody,
	lastTransactionInCollection bool,
) transactionRequest {
	txnId := txnBody.ID()
	txnIdStr := txnId.String()

	return transactionRequest{
		collectionInfo: collection,
		txnId:          txnId,
		txnIdStr:       txnIdStr,
		txnIndex:       txnIndex,
		ctx: fvm.NewContextFromParent(
			collectionCtx,
			fvm.WithLogger(
				collectionLogger.With().
					Str("tx_id", txnIdStr).
					Uint32("tx_index", txnIndex).
					Logger())),
		TransactionProcedure: fvm.NewTransaction(
			txnId,
			txnIndex,
			txnBody),
		lastTransactionInCollection: lastTransactionInCollection,
	}
}

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(
		ctx context.Context,
		parentBlockExecutionResultID flow.Identifier,
		block *entity.ExecutableBlock,
		snapshot snapshot.StorageSnapshot,
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
	maxConcurrency int,
) (BlockComputer, error) {
	if maxConcurrency < 1 {
		return nil, fmt.Errorf("invalid maxConcurrency: %d", maxConcurrency)
	}
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
	snapshot snapshot.StorageSnapshot,
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

func (e *blockComputer) queueTransactionRequests(
	blockId flow.Identifier,
	blockIdStr string,
	blockHeader *flow.Header,
	rawCollections []*entity.CompleteCollection,
	systemTxnBody *flow.TransactionBody,
	requestQueue chan transactionRequest,
) {
	txnIndex := uint32(0)

	collectionCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(blockHeader))

	for idx, collection := range rawCollections {
		collectionLogger := collectionCtx.Logger.With().
			Str("block_id", blockIdStr).
			Uint64("height", blockHeader.Height).
			Bool("system_chunk", false).
			Bool("system_transaction", false).
			Logger()

		collectionInfo := collectionInfo{
			blockId:             blockId,
			blockIdStr:          blockIdStr,
			collectionIndex:     idx,
			CompleteCollection:  collection,
			isSystemTransaction: false,
		}

		for i, txnBody := range collection.Transactions {
			requestQueue <- newTransactionRequest(
				collectionInfo,
				collectionCtx,
				collectionLogger,
				txnIndex,
				txnBody,
				i == len(collection.Transactions)-1)
			txnIndex += 1
		}

	}

	systemCtx := fvm.NewContextFromParent(
		e.systemChunkCtx,
		fvm.WithBlockHeader(blockHeader))
	systemCollectionLogger := systemCtx.Logger.With().
		Str("block_id", blockIdStr).
		Uint64("height", blockHeader.Height).
		Bool("system_chunk", true).
		Bool("system_transaction", true).
		Logger()
	systemCollectionInfo := collectionInfo{
		blockId:         blockId,
		blockIdStr:      blockIdStr,
		collectionIndex: len(rawCollections),
		CompleteCollection: &entity.CompleteCollection{
			Transactions: []*flow.TransactionBody{systemTxnBody},
		},
		isSystemTransaction: true,
	}

	requestQueue <- newTransactionRequest(
		systemCollectionInfo,
		systemCtx,
		systemCollectionLogger,
		txnIndex,
		systemTxnBody,
		true)
}

func numberOfTransactionsInBlock(collections []*entity.CompleteCollection) int {
	numTxns := 1 // there's one system transaction per block
	for _, collection := range collections {
		numTxns += len(collection.Transactions)
	}

	return numTxns
}

func (e *blockComputer) executeBlock(
	ctx context.Context,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	baseSnapshot snapshot.StorageSnapshot,
	derivedBlockData *derived.DerivedBlockData,
) (
	*execution.ComputationResult,
	error,
) {
	// check the start state is set
	if !block.HasStartState() {
		return nil, fmt.Errorf("executable block start state is not set")
	}

	blockId := block.ID()
	blockIdStr := blockId.String()

	rawCollections := block.Collections()

	blockSpan := e.tracer.StartSpanFromParent(
		e.tracer.BlockRootSpan(blockId),
		trace.EXEComputeBlock)
	blockSpan.SetAttributes(
		attribute.String("block_id", blockIdStr),
		attribute.Int("collection_counts", len(rawCollections)))
	defer blockSpan.End()

	systemTxn, err := blueprints.SystemChunkTransaction(e.vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get system chunk transaction: %w",
			err)
	}

	numTxns := numberOfTransactionsInBlock(rawCollections)

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
		numTxns,
		e.colResCons)
	defer collector.Stop()

	requestQueue := make(chan transactionRequest, numTxns)
	e.queueTransactionRequests(
		blockId,
		blockIdStr,
		block.Block.Header,
		rawCollections,
		systemTxn,
		requestQueue)
	close(requestQueue)

	database := storage.NewBlockDatabase(baseSnapshot, 0, derivedBlockData)

	for request := range requestQueue {
		request.ctx.Logger.Info().Msg("executing transaction")
		err := e.executeTransaction(
			blockSpan,
			database,
			collector,
			request)
		if err != nil {
			return nil, err
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
	database *storage.BlockDatabase,
	collector *resultCollector,
	request transactionRequest,
) error {
	txn, err := e.executeTransactionInternal(
		parentSpan,
		database,
		collector,
		request)
	if err != nil {
		prefix := ""
		if request.isSystemTransaction {
			prefix = "system "
		}

		snapshotTime := logical.Time(0)
		if txn != nil {
			snapshotTime = txn.SnapshotTime()
		}

		return fmt.Errorf(
			"failed to execute %stransaction %v (%d@%d) for block %s "+
				"at height %v: %w",
			prefix,
			request.txnIdStr,
			request.txnIndex,
			snapshotTime,
			request.blockIdStr,
			request.ctx.BlockHeader.Height,
			err)
	}

	return nil
}

func (e *blockComputer) executeTransactionInternal(
	parentSpan otelTrace.Span,
	database *storage.BlockDatabase,
	collector *resultCollector,
	request transactionRequest,
) (
	storage.Transaction,
	error,
) {
	startedAt := time.Now()

	txSpan := e.tracer.StartSampledSpanFromParent(
		parentSpan,
		request.txnId,
		trace.EXEComputeTransaction)
	txSpan.SetAttributes(
		attribute.String("tx_id", request.txnIdStr),
		attribute.Int64("tx_index", int64(request.txnIndex)),
		attribute.Int("col_index", request.collectionIndex),
	)
	defer txSpan.End()

	request.ctx = fvm.NewContextFromParent(request.ctx, fvm.WithSpan(txSpan))

	txn, err := database.NewTransaction(
		request.ExecutionTime(),
		fvm.ProcedureStateParameters(request.ctx, request))
	if err != nil {
		return nil, err
	}

	executor := e.vm.NewExecutor(request.ctx, request.TransactionProcedure, txn)
	defer executor.Cleanup()

	err = executor.Preprocess()
	if err != nil {
		return txn, err
	}

	err = executor.Execute()
	if err != nil {
		return txn, err
	}

	err = txn.Finalize()
	if err != nil {
		return txn, err
	}

	executionSnapshot, err := txn.Commit()
	if err != nil {
		return txn, err
	}

	output := executor.Output()
	collector.AddTransactionResult(
		request,
		executionSnapshot,
		output,
		time.Since(startedAt))

	return txn, nil
}
