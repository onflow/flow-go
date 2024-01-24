package computer

import (
	"context"
	"fmt"
	"sync"

	"github.com/onflow/crypto/hash"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/result"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/errors"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
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

type TransactionRequest struct {
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
) TransactionRequest {
	txnId := txnBody.ID()
	txnIdStr := txnId.String()

	return TransactionRequest{
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
	executionDataProvider provider.Provider
	signer                module.Local
	spockHasher           hash.Hasher
	receiptHasher         hash.Hasher
	colResCons            []result.ExecutedCollectionConsumer
	protocolState         protocol.State
	maxConcurrency        int
}

func SystemChunkContext(vmCtx fvm.Context) fvm.Context {
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
		// only the system transaction is allowed to call the block entropy provider
		fvm.WithRandomSourceHistoryCallAllowed(true),
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
	executionDataProvider provider.Provider,
	colResCons []result.ExecutedCollectionConsumer,
	state protocol.State,
	maxConcurrency int,
) (BlockComputer, error) {
	if maxConcurrency < 1 {
		return nil, fmt.Errorf("invalid maxConcurrency: %d", maxConcurrency)
	}
	systemChunkCtx := SystemChunkContext(vmCtx)
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
		protocolState:         state,
		maxConcurrency:        maxConcurrency,
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
	requestQueue chan TransactionRequest,
	numTxns int,
) {
	txnIndex := uint32(0)

	collectionCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(blockHeader),
		// `protocol.Snapshot` implements `EntropyProvider` interface
		// Note that `Snapshot` possible errors for RandomSource() are:
		// - storage.ErrNotFound if the QC is unknown.
		// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
		// However, at this stage, snapshot reference block should be known and the QC should also be known,
		// so no error is expected in normal operations, as required by `EntropyProvider`.
		fvm.WithEntropyProvider(e.protocolState.AtBlockID(blockId)),
	)

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
		fvm.WithBlockHeader(blockHeader),
		// `protocol.Snapshot` implements `EntropyProvider` interface
		// Note that `Snapshot` possible errors for RandomSource() are:
		// - storage.ErrNotFound if the QC is unknown.
		// - state.ErrUnknownSnapshotReference if the snapshot reference block is unknown
		// However, at this stage, snapshot reference block should be known and the QC should also be known,
		// so no error is expected in normal operations, as required by `EntropyProvider`.
		fvm.WithEntropyProvider(e.protocolState.AtBlockID(blockId)),
	)
	systemCollectionLogger := systemCtx.Logger.With().
		Str("block_id", blockIdStr).
		Uint64("height", blockHeader.Height).
		Bool("system_chunk", true).
		Bool("system_transaction", true).
		Int("num_collections", len(rawCollections)).
		Int("num_txs", numTxns).
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
		e.colResCons,
		baseSnapshot,
	)
	defer collector.Stop()

	requestQueue := make(chan TransactionRequest, numTxns)

	database := newTransactionCoordinator(
		e.vm,
		baseSnapshot,
		derivedBlockData,
		collector)

	e.queueTransactionRequests(
		blockId,
		blockIdStr,
		block.Block.Header,
		rawCollections,
		systemTxn,
		requestQueue,
		numTxns,
	)
	close(requestQueue)

	wg := &sync.WaitGroup{}
	wg.Add(e.maxConcurrency)

	for i := 0; i < e.maxConcurrency; i++ {
		go e.executeTransactions(
			blockSpan,
			database,
			requestQueue,
			wg)
	}

	wg.Wait()

	err = database.Error()
	if err != nil {
		return nil, err
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

func (e *blockComputer) executeTransactions(
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	requestQueue chan TransactionRequest,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for request := range requestQueue {
		attempt := 0
		for {
			request.ctx.Logger.Info().
				Int("attempt", attempt).
				Msg("executing transaction")

			attempt += 1
			err := e.executeTransaction(blockSpan, database, request, attempt)

			if errors.IsRetryableConflictError(err) {
				request.ctx.Logger.Info().
					Int("attempt", attempt).
					Str("conflict_error", err.Error()).
					Msg("conflict detected. retrying transaction")
				continue
			}

			if err != nil {
				database.AbortAllOutstandingTransactions(err)
				return
			}

			break // process next transaction
		}
	}
}

func (e *blockComputer) executeTransaction(
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	request TransactionRequest,
	attempt int,
) error {
	txn, err := e.executeTransactionInternal(
		blockSpan,
		database,
		request,
		attempt)
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
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	request TransactionRequest,
	attempt int,
) (
	*transaction,
	error,
) {
	txSpan := e.tracer.StartSampledSpanFromParent(
		blockSpan,
		request.txnId,
		trace.EXEComputeTransaction)
	txSpan.SetAttributes(
		attribute.String("tx_id", request.txnIdStr),
		attribute.Int64("tx_index", int64(request.txnIndex)),
		attribute.Int("col_index", request.collectionIndex),
	)
	defer txSpan.End()

	request.ctx = fvm.NewContextFromParent(request.ctx, fvm.WithSpan(txSpan))

	txn, err := database.NewTransaction(request, attempt)
	if err != nil {
		return nil, err
	}
	defer txn.Cleanup()

	err = txn.Preprocess()
	if err != nil {
		return txn, err
	}

	// Validating here gives us an opportunity to early abort/retry the
	// transaction in case the conflict is detectable after preprocessing.
	// This is strictly an optimization and hence we don't need to wait for
	// updates (removing this validate call won't impact correctness).
	err = txn.Validate()
	if err != nil {
		return txn, err
	}

	err = txn.Execute()
	if err != nil {
		return txn, err
	}

	err = txn.Finalize()
	if err != nil {
		return txn, err
	}

	// Snapshot time smaller than execution time indicates there are outstanding
	// transaction(s) that must be committed before this transaction can be
	// committed.
	for txn.SnapshotTime() < request.ExecutionTime() {
		err = txn.WaitForUpdates()
		if err != nil {
			return txn, err
		}

		err = txn.Validate()
		if err != nil {
			return txn, err
		}
	}

	return txn, txn.Commit()
}
