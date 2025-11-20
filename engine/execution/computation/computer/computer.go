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
}

type ComputerTransactionType uint

const (
	ComputerTransactionTypeUser ComputerTransactionType = iota
	ComputerTransactionTypeSystem
	ComputerTransactionTypeScheduled
)

type TransactionRequest struct {
	collectionInfo

	txnId    flow.Identifier
	txnIdStr string

	txnIndex uint32

	transactionType             ComputerTransactionType
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
	transactionType ComputerTransactionType,
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
		transactionType:             transactionType,
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
	systemChunkCtx        fvm.Context
	callbackCtx           fvm.Context
	metrics               module.ExecutionMetrics
	tracer                module.Tracer
	log                   zerolog.Logger
	systemTxn             *flow.TransactionBody
	processCallbackTxn    *flow.TransactionBody
	committer             ViewCommitter
	executionDataProvider provider.Provider
	signer                module.Local
	spockHasher           hash.Hasher
	receiptHasher         hash.Hasher
	colResCons            []result.ExecutedCollectionConsumer
	protocolState         protocol.SnapshotExecutionSubsetProvider
	maxConcurrency        int
}

// SystemChunkContext is the context for the system chunk transaction.
func SystemChunkContext(vmCtx fvm.Context, metrics module.ExecutionMetrics) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithMetricsReporter(metrics),
		fvm.WithContractDeploymentRestricted(false),
		fvm.WithContractRemovalRestricted(false),
		fvm.WithEventCollectionSizeLimit(SystemChunkEventCollectionMaxSize),
		fvm.WithMemoryAndInteractionLimitsDisabled(),
		// only the system transaction is allowed to call the block entropy provider
		fvm.WithRandomSourceHistoryCallAllowed(true),
		fvm.WithAccountStorageLimit(false),
	)
}

// ScheduledTransactionContext is the context for the scheduled transactions.
func ScheduledTransactionContext(vmCtx fvm.Context, metrics module.ExecutionMetrics) fvm.Context {
	return fvm.NewContextFromParent(
		vmCtx,
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		fvm.WithTransactionFeesEnabled(false),
		fvm.WithMetricsReporter(metrics),
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

	vmCtx = fvm.NewContextFromParent(
		vmCtx,
		fvm.WithMetricsReporter(metrics),
		fvm.WithTracer(tracer))

	systemTxn, err := blueprints.SystemChunkTransaction(vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf("could not build system chunk transaction: %w", err)
	}

	processCallbackTxn, err := blueprints.ProcessCallbacksTransaction(vmCtx.Chain)
	if err != nil {
		return nil, fmt.Errorf("failed to generate callbacks script: %w", err)
	}

	return &blockComputer{
		vm:                    vm,
		vmCtx:                 vmCtx,
		callbackCtx:           ScheduledTransactionContext(vmCtx, metrics),
		systemChunkCtx:        SystemChunkContext(vmCtx, metrics),
		metrics:               metrics,
		tracer:                tracer,
		log:                   logger,
		systemTxn:             systemTxn,
		processCallbackTxn:    processCallbackTxn,
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
		count += len(collection.Collection.Transactions)
	}

	return count
}

// queueUserTransactions enqueues transaction processing requests for all user and
// system transactions in the given block.
//
// If constructing the Collection for the system transaction fails (for example, because
// NewCollection returned an error due to invalid input), this method returns an
// irrecoverable exception. Such an error indicates a fatal, unexpected condition and
// should abort block execution.
//
// Returns:
//   - nil on success,
//   - error if an irrecoverable error is received if the system transaction’s Collection cannot be constructed.
func (e *blockComputer) queueUserTransactions(
	blockId flow.Identifier,
	blockHeader *flow.Header,
	rawCollections []*entity.CompleteCollection,
	userTxCount int,
) chan TransactionRequest {
	txQueue := make(chan TransactionRequest, userTxCount)
	defer close(txQueue)

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
			blockId:            blockId,
			blockIdStr:         blockIdStr,
			blockHeight:        blockHeader.Height,
			collectionIndex:    idx,
			CompleteCollection: collection,
		}

		for i, txnBody := range collection.Collection.Transactions {
			txQueue <- newTransactionRequest(
				collectionInfo,
				collectionCtx,
				collectionLogger,
				txnIndex,
				txnBody,
				ComputerTransactionTypeUser,
				i == len(collection.Collection.Transactions)-1,
			)
			txnIndex += 1
		}
	}

	return txQueue
}

func (e *blockComputer) queueSystemTransactions(
	callbackCtx fvm.Context,
	systemChunkCtx fvm.Context,
	systemCollection collectionInfo,
	systemTxn *flow.TransactionBody,
	executeCallbackTxs []*flow.TransactionBody,
	txnIndex uint32,
	systemLogger zerolog.Logger,
) chan TransactionRequest {

	systemTxLogger := systemLogger.With().
		Uint32("num_txs", uint32(len(systemCollection.CompleteCollection.Collection.Transactions))).
		Bool("system_transaction", true).
		Logger()

	scheduledTxLogger := systemLogger.With().
		Uint32("num_txs", uint32(len(systemCollection.CompleteCollection.Collection.Transactions))).
		Bool("scheduled_transaction", true).
		Logger()

	txQueue := make(chan TransactionRequest, len(executeCallbackTxs)+1)
	defer close(txQueue)

	for _, txBody := range executeCallbackTxs {
		txQueue <- newTransactionRequest(
			systemCollection,
			callbackCtx,
			scheduledTxLogger,
			txnIndex,
			txBody,
			ComputerTransactionTypeScheduled,
			false,
		)

		txnIndex++
	}

	txQueue <- newTransactionRequest(
		systemCollection,
		systemChunkCtx,
		systemTxLogger,
		txnIndex,
		systemTxn,
		ComputerTransactionTypeSystem,
		true,
	)

	return txQueue
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
		e.tracer.BlockRootSpan(block.BlockID()),
		trace.EXEComputeBlock)
	blockSpan.SetAttributes(
		attribute.String("block_id", block.BlockID().String()),
		attribute.Int("collection_counts", len(rawCollections)))
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
		// Add buffer just in case result collection becomes slower than the execution
		e.maxConcurrency*2,
		e.colResCons,
		baseSnapshot,
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

	err := e.executeSystemTransactions(
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
		Str("block_id", block.BlockID().String()).
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
	txQueue := e.queueUserTransactions(
		block.BlockID(),
		block.Block.ToHeader(),
		rawCollections,
		userTxCount,
	)

	e.executeQueue(blockSpan, database, txQueue)
}

// executeSystemTransactions executes all system transactions in the block as part of the system collection.
//
// When scheduled callbacks are enabled, system transactions are executed in the following order:
//  1. Process scheduled transactions transaction - queries the scheduler contract to identify ready transactions
//     and emits events containing transaction IDs and execution effort requirements
//  2. Scheduled transaction execution transactions - one transaction per transaction ID from step 1 events,
//     each executing a single scheduled transaction with its specified effort limit
//  3. System chunk transaction - performs standard system operations
//
// When scheduled callbacks are disabled, only the system chunk transaction is executed.
//
// All errors are indicators of bugs or corrupted internal state (continuation impossible)
func (e *blockComputer) executeSystemTransactions(
	block *entity.ExecutableBlock,
	blockSpan otelTrace.Span,
	database *transactionCoordinator,
	rawCollections []*entity.CompleteCollection,
	userTxCount int,
) error {
	userCollectionCount := len(rawCollections)
	txIndex := uint32(userTxCount)

	callbackCtx := fvm.NewContextFromParent(
		e.callbackCtx,
		fvm.WithBlockHeader(block.Block.ToHeader()),
		fvm.WithProtocolStateSnapshot(e.protocolState.AtBlockID(block.BlockID())),
	)

	systemChunkCtx := fvm.NewContextFromParent(
		e.systemChunkCtx,
		fvm.WithBlockHeader(block.Block.ToHeader()),
		fvm.WithProtocolStateSnapshot(e.protocolState.AtBlockID(block.BlockID())),
	)

	systemCollectionInfo := collectionInfo{
		blockId:            block.BlockID(),
		blockIdStr:         block.BlockID().String(),
		blockHeight:        block.Block.Height,
		collectionIndex:    userCollectionCount,
		CompleteCollection: nil, // We do not yet know all the scheduled callbacks, so postpone construction of the collection.
	}

	systemChunkLogger := callbackCtx.Logger.With().
		Str("block_id", block.BlockID().String()).
		Uint64("height", block.Block.Height).
		Bool("system_chunk", true).
		Int("num_collections", userCollectionCount).
		Logger()

	var callbackTxs []*flow.TransactionBody

	if e.vmCtx.ScheduledTransactionsEnabled {
		// We pass in the `systemCollectionInfo` here. However, note that at this point, the composition of the system chunk
		// is not yet known. Specifically, the `entity.CompleteCollection` represents the *final* output of a process and is
		// immutable by protocol mandate. If we had a bug in our software that accidentally illegally mutated such structs,
		// likely the node encountering that bug would misbehave and get slashed, or in the worst case the flow protocol might
		// be compromised. Therefore, we have the rigorous convention in our code base that the `CompleteCollection` is only
		// constructed once the final composition of the system chunk has been determined.
		// To that end, the CompleteCollection is nil here, such that any attempt to access the Collection will panic.
		callbacks, updatedTxnIndex, err := e.executeProcessCallback(
			callbackCtx,
			systemCollectionInfo,
			database,
			blockSpan,
			txIndex,
			systemChunkLogger,
		)
		if err != nil {
			return err
		}

		callbackTxs = callbacks
		txIndex = updatedTxnIndex

		finalCollection, err := flow.NewCollection(flow.UntrustedCollection{
			Transactions: append(append([]*flow.TransactionBody{e.processCallbackTxn}, callbackTxs...), e.systemTxn),
		})
		if err != nil {
			return err
		}
		systemCollectionInfo.CompleteCollection = &entity.CompleteCollection{
			Collection: finalCollection,
		}
	} else {
		finalCollection, err := flow.NewCollection(flow.UntrustedCollection{
			Transactions: []*flow.TransactionBody{e.systemTxn},
		})
		if err != nil {
			return err
		}
		systemCollectionInfo.CompleteCollection = &entity.CompleteCollection{
			Collection: finalCollection,
		}
	}

	txQueue := e.queueSystemTransactions(
		callbackCtx,
		systemChunkCtx,
		systemCollectionInfo,
		e.systemTxn,
		callbackTxs,
		txIndex,
		systemChunkLogger,
	)

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

// executeProcessCallback submits a transaction that invokes the `process` method
// on the transaction scheduler contract.
//
// The `process` method scans for scheduled transactions and emits an event for each that should
// be executed. These emitted events are used to construct scheduled transaction execution transactions,
// which are then added to the system transaction collection.
//
// If the `process` transaction fails, a fatal error is returned.
//
// Note: this transaction is executed serially and not concurrently with the system transaction.
// This is because it's unclear whether the scheduled transaction executions triggered by `process`
// will result in additional system transactions.
// In theory, if no additional transactions are emitted, concurrent execution could be optimized.
// However, due to the added complexity, this optimization was deferred.
func (e *blockComputer) executeProcessCallback(
	systemCtx fvm.Context,
	systemCollectionInfo collectionInfo,
	database *transactionCoordinator,
	blockSpan otelTrace.Span,
	txnIndex uint32,
	systemLogger zerolog.Logger,
) ([]*flow.TransactionBody, uint32, error) {
	callbackLogger := systemLogger.With().
		Bool("system_transaction", true).
		Logger()

	request := newTransactionRequest(
		systemCollectionInfo,
		systemCtx,
		callbackLogger,
		txnIndex,
		e.processCallbackTxn,
		ComputerTransactionTypeSystem,
		false)

	txnIndex++

	txn, err := e.executeTransactionInternal(blockSpan, database, request, 0)
	if err != nil {
		snapshotTime := logical.Time(0)
		if txn != nil {
			snapshotTime = txn.SnapshotTime()
		}

		return nil, 0, fmt.Errorf(
			"failed to execute system process transaction %v (%d@%d) for block %s at height %v: %w",
			request.txnIdStr,
			request.txnIndex,
			snapshotTime,
			request.blockIdStr,
			request.ctx.BlockHeader.Height,
			err)
	}

	if txn.Output().Err != nil {
		// if the process transaction fails we log the critical error but don't return an error
		// so that block execution continues and only the scheduled transactions halt
		callbackLogger.Error().
			Err(txn.Output().Err).
			Bool("critical_error", true).
			Uint64("height", request.ctx.BlockHeader.Height).
			Msg("system process transaction output error")

		return nil, txnIndex, nil
	}

	callbackTxs, err := blueprints.ExecuteCallbacksTransactions(e.vmCtx.Chain, txn.Output().Events)
	if err != nil {
		return nil, 0, err
	}

	if len(callbackTxs) > 0 {
		// calculate total computation limits for execute scheduled transaction transactions
		var totalExecuteComputationLimits uint64
		for _, tx := range callbackTxs {
			totalExecuteComputationLimits += tx.GasLimit
		}

		// report metrics for scheduled transactions executed
		e.metrics.ExecutionScheduledTransactionsExecuted(
			len(callbackTxs),
			txn.Output().ComputationUsed,
			totalExecuteComputationLimits,
		)
	}

	return callbackTxs, txnIndex, nil
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
		if request.transactionType == ComputerTransactionTypeSystem {
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
