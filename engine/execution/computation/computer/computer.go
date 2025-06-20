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
	blockId     flow.Identifier
	blockIdStr  string
	blockHeight uint64

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
	systemTxn             *flow.TransactionBody
	committer             ViewCommitter
	executionDataProvider provider.Provider
	signer                module.Local
	spockHasher           hash.Hasher
	receiptHasher         hash.Hasher
	colResCons            []result.ExecutedCollectionConsumer
	protocolState         protocol.SnapshotExecutionSubsetProvider
	maxConcurrency        int
}

func SystemChunkContext(vmCtx fvm.Context, metrics module.ExecutionMetrics) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(false),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithEventCollectionSizeLimit(SystemChunkEventCollectionMaxSize),
		fvm.WithMemoryAndInteractionLimitsDisabled(),
		// only the system transaction is allowed to call the block entropy provider
		fvm.WithRandomSourceHistoryCallAllowed(true),
		fvm.WithMetricsReporter(metrics),
		fvm.WithAccountStorageLimit(false),
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
	state protocol.SnapshotExecutionSubsetProvider,
	maxConcurrency int,
) (BlockComputer, error) {
	if maxConcurrency < 1 {
		return nil, fmt.Errorf("invalid maxConcurrency: %d", maxConcurrency)
	}

	// this is a safeguard to prevent scripts from writing to the program cache on Execution nodes.
	// writes are only allowed by transactions.
	if vmCtx.AllowProgramCacheWritesInScripts {
		return nil, fmt.Errorf("program cache writes are not allowed in scripts on Execution nodes")
	}

	systemChunkCtx := SystemChunkContext(vmCtx, metrics)
	vmCtx = fvm.NewContextFromParent(
		vmCtx,
		fvm.WithMetricsReporter(metrics),
		fvm.WithTracer(tracer))

	systemTxn, err := blueprints.SystemChunkTransaction(vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	return &blockComputer{
		vm:                    vm,
		vmCtx:                 vmCtx,
		metrics:               metrics,
		tracer:                tracer,
		log:                   logger,
		systemChunkCtx:        systemChunkCtx,
		systemTxn:             systemTxn,
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

func (e *blockComputer) userTransactionsCount(collections []*entity.CompleteCollection) int {
	count := 0
	for _, collection := range collections {
		count += len(collection.Transactions)
	}

	return count
}

func (e *blockComputer) queueUserTransactions(
	blockId flow.Identifier,
	blockHeader *flow.Header,
	rawCollections []*entity.CompleteCollection,
	requestQueue chan TransactionRequest,
) {
	txnIndex := uint32(0)
	blockIdStr := blockId.String()

	collectionCtx := fvm.NewContextFromParent(
		e.vmCtx,
		fvm.WithBlockHeader(blockHeader),
		fvm.WithProtocolStateSnapshot(e.protocolState.AtBlockID(blockId)),
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
			blockHeight:         blockHeader.Height,
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
}

func (e *blockComputer) queueSystemTransaction(
	systemCtx fvm.Context,
	systemCollectionInfo collectionInfo,
	systemTxn *flow.TransactionBody,
	executeCallbackTxs []*flow.TransactionBody,
	requestQueue chan TransactionRequest,
	txnIndex uint32,
	systemLogger zerolog.Logger,
) {
	allTxs := append(executeCallbackTxs, systemTxn)
	systemCollectionInfo.CompleteCollection.Transactions = allTxs
	systemLogger = systemLogger.With().Uint32("num_txs", uint32(len(allTxs))).Logger()

	for i, txBody := range allTxs {
		last := i == len(allTxs)-1

		requestQueue <- newTransactionRequest(
			systemCollectionInfo,
			systemCtx,
			systemLogger,
			txnIndex,
			txBody,
			last,
		)

		txnIndex++
	}
}

// selectChunkConstructorForProtocolVersion selects a [flow.Chunk] constructor to
// use when constructing the [flow.ExecutionResult] for the input block. We select
// based on the protocol version at the input block. When we process the version upgrade
// event to protocol version 2, we begin populating the new [flow.ChunkBody.ServiceEventCount]
// field.
// Deprecated:
// TODO(mainnet27, #6773): remove this function https://github.com/onflow/flow-go/issues/6773
func (e *blockComputer) selectChunkConstructorForProtocolVersion(blockID flow.Identifier) (flow.ChunkConstructor, error) {
	ps, err := e.protocolState.AtBlockID(blockID).ProtocolState()
	if err != nil {
		return nil, err
	}
	version := ps.GetProtocolStateVersion()
	if version < 2 {
		return flow.NewChunk_ProtocolVersion1, nil
	} else {
		return flow.NewChunk, nil
	}
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

	rawCollections := block.Collections()
	userTxCount := e.userTransactionsCount(rawCollections)

	blockSpan := e.tracer.StartSpanFromParent(
		e.tracer.BlockRootSpan(block.ID()),
		trace.EXEComputeBlock)
	blockSpan.SetAttributes(
		attribute.String("block_id", block.ID().String()),
		attribute.Int("collection_counts", len(rawCollections)))
	defer blockSpan.End()

	// We temporarily support chunk models associated with both protocol versions 1 and 2.
	// TODO(mainnet27, #6773): remove this https://github.com/onflow/flow-go/issues/6773
	versionedChunkConstructor, err := e.selectChunkConstructorForProtocolVersion(block.ID())
	if err != nil {
		return nil, fmt.Errorf("could not select chunk constructor for current protocol version: %w", err)
	}

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
		e.maxConcurrency*2, // we add some buffer just in case result collection becomes slower than the execution
		e.colResCons,
		baseSnapshot,
		versionedChunkConstructor,
	)
	defer collector.Stop()

	database := newTransactionCoordinator(
		e.vm,
		baseSnapshot,
		derivedBlockData,
		collector)

	e.executeUserTransactions(
		block,
		blockSpan,
		database,
		rawCollections,
		userTxCount,
	)

	err = e.executeSystemTransactions(
		block,
		blockSpan,
		database,
		rawCollections,
		userTxCount,
	)
	if err != nil {
		return nil, err
	}

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

// executeUserTransactions executes the user transactions in the block.
// It queues the user transactions into a request queue and then executes them in parallel.
func (e *blockComputer) executeUserTransactions(
	block *entity.ExecutableBlock,
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	rawCollections []*entity.CompleteCollection,
	userTxCount int,
) {
	txQueue := make(chan TransactionRequest, userTxCount)

	e.queueUserTransactions(
		block.ID(),
		block.Block.Header,
		rawCollections,
		txQueue,
	)

	close(txQueue)

	e.executeQueue(blockSpan, database, txQueue)
}

// executeSystemTransactions executes all system transactions in the block as part of the system collection.
//
// System transactions are executed in the following order:
// 1. system transaction that processes the scheduled callbacks which is a blocking transaction and
// the result is used for the next system transaction
// 2. system transactions that each execute a single scheduled callback by the ID obtained from events
// of the previous system transaction
// 3. system transaction that executes the system chunk
//
// An error can be returned if the process callback transaction fails. This is a fatal error.
func (e *blockComputer) executeSystemTransactions(
	block *entity.ExecutableBlock,
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	rawCollections []*entity.CompleteCollection,
	userTxCount int,
) error {
	userCollectionCount := len(rawCollections)
	txIndex := uint32(userTxCount)

	systemCtx := fvm.NewContextFromParent(
		e.systemChunkCtx,
		fvm.WithBlockHeader(block.Block.Header),
		fvm.WithProtocolStateSnapshot(e.protocolState.AtBlockID(block.ID())),
	)

	systemLogger := systemCtx.Logger.With().
		Str("block_id", block.ID().String()).
		Uint64("height", block.Block.Header.Height).
		Bool("system_chunk", true).
		Bool("system_transaction", true).
		Int("num_collections", userCollectionCount).
		Logger()

	systemCollectionInfo := collectionInfo{
		blockId:         block.ID(),
		blockIdStr:      block.ID().String(),
		blockHeight:     block.Block.Header.Height,
		collectionIndex: len(rawCollections),
		CompleteCollection: &entity.CompleteCollection{
			Transactions: []*flow.TransactionBody{},
		},
		isSystemTransaction: true,
	}

	var callbackTxs []*flow.TransactionBody
	var err error

	if e.vmCtx.ScheduleCallbacksEnabled {
		callbackTxs, err = e.executeProcessCallback(
			systemCtx,
			systemCollectionInfo,
			database,
			blockSpan,
			txIndex,
			systemLogger,
		)
		if err != nil {
			return err
		}

		txIndex++
	}

	const systemTxCount = 1 // we always have one system transaction
	txQueue := make(chan TransactionRequest, len(callbackTxs)+systemTxCount)

	e.queueSystemTransaction(
		systemCtx,
		systemCollectionInfo,
		e.systemTxn,
		callbackTxs,
		txQueue,
		txIndex,
		systemLogger,
	)

	close(txQueue)

	e.executeQueue(blockSpan, database, txQueue)

	return nil
}

// executeQueue executes the transactions in the request queue in parallel with the maxConcurrency workers.
func (e *blockComputer) executeQueue(
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	txQueue chan TransactionRequest,
) {
	wg := &sync.WaitGroup{}
	wg.Add(e.maxConcurrency)

	for range e.maxConcurrency {
		go e.executeTransactions(
			blockSpan,
			database,
			txQueue,
			wg)
	}

	wg.Wait()
}

// executeProcessCallback executes a transaction that calls callback scheduler contract process method.
// The execution result contains events that are emitted for each callback which is ready for execution.
// We use these events to prepare callback execution transactions, which are later executed as part of the system collection.
// An error can be returned if the process callback transaction fails. This is a fatal error.
func (e *blockComputer) executeProcessCallback(
	systemCtx fvm.Context,
	systemCollectionInfo collectionInfo,
	database *transactionCoordinator,
	blockSpan otelTrace.Span,
	txnIndex uint32,
	systemLogger zerolog.Logger,
) ([]*flow.TransactionBody, error) {
	processTxn := blueprints.ProcessCallbacksTransaction(e.vmCtx.Chain)

	request := newTransactionRequest(
		systemCollectionInfo,
		systemCtx,
		systemLogger,
		txnIndex,
		processTxn,
		false)

	txn, err := e.executeTransactionInternal(blockSpan, database, request, 0)
	if err != nil {
		snapshotTime := logical.Time(0)
		if txn != nil {
			snapshotTime = txn.SnapshotTime()
		}

		return nil, fmt.Errorf(
			"failed to execute %s transaction %v (%d@%d) for block %s at height %v: %w",
			"system",
			request.txnIdStr,
			request.txnIndex,
			snapshotTime,
			request.blockIdStr,
			request.ctx.BlockHeader.Height,
			err)
	}

	if txn.Output().Err != nil {
		return nil, fmt.Errorf(
			"process callback transaction %s error: %v",
			request.txnIdStr,
			txn.Output().Err)
	}

	return blueprints.ExecuteCallbacksTransactions(e.vmCtx.Chain, txn.Output().Events)
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
