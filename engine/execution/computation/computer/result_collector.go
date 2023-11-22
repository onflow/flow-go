package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/result"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
)

// ViewCommitter commits execution snapshot to the ledger and collects
// the proofs
type ViewCommitter interface {
	// CommitView commits an execution snapshot and collects proofs
	CommitView(
		*snapshot.ExecutionSnapshot,
		execution.ExtendableStorageSnapshot,
	) (
		flow.StateCommitment, // TODO(leo): deprecate. see storehouse.ExtendableStorageSnapshot.Commitment()
		[]byte,
		*ledger.TrieUpdate,
		execution.ExtendableStorageSnapshot,
		error,
	)
}

type transactionResult struct {
	TransactionRequest
	*snapshot.ExecutionSnapshot
	fvm.ProcedureOutput
	timeSpent          time.Duration
	numConflictRetries int
}

// TODO(ramtin): move committer and other folks to consumers layer
type resultCollector struct {
	tracer    module.Tracer
	blockSpan otelTrace.Span

	metrics module.ExecutionMetrics

	closeOnce          sync.Once
	processorInputChan chan transactionResult
	processorDoneChan  chan struct{}
	processorError     error

	committer ViewCommitter

	signer        module.Local
	spockHasher   hash.Hasher
	receiptHasher hash.Hasher

	executionDataProvider provider.Provider

	parentBlockExecutionResultID flow.Identifier

	result    *execution.ComputationResult
	consumers []result.ExecutedCollectionConsumer

	spockSignatures []crypto.Signature

	blockStartTime time.Time
	blockStats     module.ExecutionResultStats
	blockMeter     *meter.Meter

	currentCollectionStartTime       time.Time
	currentCollectionState           *state.ExecutionState
	currentCollectionStats           module.ExecutionResultStats
	currentCollectionStorageSnapshot execution.ExtendableStorageSnapshot
}

func newResultCollector(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	committer ViewCommitter,
	signer module.Local,
	executionDataProvider provider.Provider,
	spockHasher hash.Hasher,
	receiptHasher hash.Hasher,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	numTransactions int,
	consumers []result.ExecutedCollectionConsumer,
	previousBlockSnapshot snapshot.StorageSnapshot,
) *resultCollector {
	numCollections := len(block.Collections()) + 1
	now := time.Now()
	collector := &resultCollector{
		tracer:                       tracer,
		blockSpan:                    blockSpan,
		metrics:                      metrics,
		processorInputChan:           make(chan transactionResult, numTransactions),
		processorDoneChan:            make(chan struct{}),
		committer:                    committer,
		signer:                       signer,
		spockHasher:                  spockHasher,
		receiptHasher:                receiptHasher,
		executionDataProvider:        executionDataProvider,
		parentBlockExecutionResultID: parentBlockExecutionResultID,
		result:                       execution.NewEmptyComputationResult(block),
		consumers:                    consumers,
		spockSignatures:              make([]crypto.Signature, 0, numCollections),
		blockStartTime:               now,
		blockMeter:                   meter.NewMeter(meter.DefaultParameters()),
		currentCollectionStartTime:   now,
		currentCollectionState:       state.NewExecutionState(nil, state.DefaultParameters()),
		currentCollectionStats: module.ExecutionResultStats{
			NumberOfCollections: 1,
		},
		currentCollectionStorageSnapshot: storehouse.NewExecutingBlockSnapshot(
			previousBlockSnapshot,
			*block.StartState,
		),
	}

	go collector.runResultProcessor()

	return collector
}

func (collector *resultCollector) commitCollection(
	collection collectionInfo,
	startTime time.Time,
	collectionExecutionSnapshot *snapshot.ExecutionSnapshot,
) error {
	defer collector.tracer.StartSpanFromParent(
		collector.blockSpan,
		trace.EXECommitDelta).End()

	startState := collector.currentCollectionStorageSnapshot.Commitment()

	_, proof, trieUpdate, newSnapshot, err := collector.committer.CommitView(
		collectionExecutionSnapshot,
		collector.currentCollectionStorageSnapshot,
	)
	if err != nil {
		return fmt.Errorf("commit view failed: %w", err)
	}

	endState := newSnapshot.Commitment()
	collector.currentCollectionStorageSnapshot = newSnapshot

	execColRes := collector.result.CollectionExecutionResultAt(collection.collectionIndex)
	execColRes.UpdateExecutionSnapshot(collectionExecutionSnapshot)

	events := execColRes.Events()
	eventsHash, err := flow.EventsMerkleRootHash(events)
	if err != nil {
		return fmt.Errorf("hash events failed: %w", err)
	}

	txResults := execColRes.TransactionResults()
	convertedTxResults := execution_data.ConvertTransactionResults(txResults)

	col := collection.Collection()
	chunkExecData := &execution_data.ChunkExecutionData{
		Collection:         &col,
		Events:             events,
		TrieUpdate:         trieUpdate,
		TransactionResults: convertedTxResults,
	}

	collector.result.AppendCollectionAttestationResult(
		startState,
		endState,
		proof,
		eventsHash,
		chunkExecData,
	)

	collector.metrics.ExecutionChunkDataPackGenerated(
		len(proof),
		len(collection.Transactions))

	spock, err := collector.signer.SignFunc(
		collectionExecutionSnapshot.SpockSecret,
		collector.spockHasher,
		SPOCKProve)
	if err != nil {
		return fmt.Errorf("signing spock hash failed: %w", err)
	}

	collector.spockSignatures = append(collector.spockSignatures, spock)

	collector.currentCollectionStats.EventCounts = len(events)
	collector.currentCollectionStats.EventSize = events.ByteSize()
	collector.currentCollectionStats.NumberOfRegistersTouched = len(
		collectionExecutionSnapshot.AllRegisterIDs())
	for _, entry := range collectionExecutionSnapshot.UpdatedRegisters() {
		collector.currentCollectionStats.NumberOfBytesWrittenToRegisters += len(
			entry.Value)
	}

	collector.metrics.ExecutionCollectionExecuted(
		time.Since(startTime),
		collector.currentCollectionStats)

	collector.blockStats.Merge(collector.currentCollectionStats)
	collector.blockMeter.MergeMeter(collectionExecutionSnapshot.Meter)

	collector.currentCollectionStartTime = time.Now()
	collector.currentCollectionState = state.NewExecutionState(nil, state.DefaultParameters())
	collector.currentCollectionStats = module.ExecutionResultStats{
		NumberOfCollections: 1,
	}

	for _, consumer := range collector.consumers {
		err = consumer.OnExecutedCollection(collector.result.CollectionExecutionResultAt(collection.collectionIndex))
		if err != nil {
			return fmt.Errorf("consumer failed: %w", err)
		}
	}

	return nil
}

func (collector *resultCollector) processTransactionResult(
	txn TransactionRequest,
	txnExecutionSnapshot *snapshot.ExecutionSnapshot,
	output fvm.ProcedureOutput,
	timeSpent time.Duration,
	numConflictRetries int,
) error {
	logger := txn.ctx.Logger.With().
		Uint64("computation_used", output.ComputationUsed).
		Uint64("memory_used", output.MemoryEstimate).
		Int64("time_spent_in_ms", timeSpent.Milliseconds()).
		Logger()

	if output.Err != nil {
		logger = logger.With().
			Str("error_message", output.Err.Error()).
			Uint16("error_code", uint16(output.Err.Code())).
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

	collector.metrics.ExecutionTransactionExecuted(
		timeSpent,
		numConflictRetries,
		output.ComputationUsed,
		output.MemoryEstimate,
		len(output.Events),
		flow.EventsList(output.Events).ByteSize(),
		output.Err != nil,
	)

	txnResult := flow.TransactionResult{
		TransactionID:   txn.ID,
		ComputationUsed: output.ComputationUsed,
		MemoryUsed:      output.MemoryEstimate,
	}
	if output.Err != nil {
		txnResult.ErrorMessage = output.Err.Error()
	}

	collector.result.
		CollectionExecutionResultAt(txn.collectionIndex).
		AppendTransactionResults(
			output.Events,
			output.ServiceEvents,
			output.ConvertedServiceEvents,
			txnResult,
		)

	err := collector.currentCollectionState.Merge(txnExecutionSnapshot)
	if err != nil {
		return fmt.Errorf("failed to merge into collection view: %w", err)
	}

	collector.currentCollectionStats.ComputationUsed += output.ComputationUsed
	collector.currentCollectionStats.MemoryUsed += output.MemoryEstimate
	collector.currentCollectionStats.NumberOfTransactions += 1

	if !txn.lastTransactionInCollection {
		return nil
	}

	return collector.commitCollection(
		txn.collectionInfo,
		collector.currentCollectionStartTime,
		collector.currentCollectionState.Finalize())
}

func (collector *resultCollector) AddTransactionResult(
	request TransactionRequest,
	snapshot *snapshot.ExecutionSnapshot,
	output fvm.ProcedureOutput,
	timeSpent time.Duration,
	numConflictRetries int,
) {
	result := transactionResult{
		TransactionRequest: request,
		ExecutionSnapshot:  snapshot,
		ProcedureOutput:    output,
		timeSpent:          timeSpent,
		numConflictRetries: numConflictRetries,
	}

	select {
	case collector.processorInputChan <- result:
		// Do nothing
	case <-collector.processorDoneChan:
		// Processor exited (probably due to an error)
	}
}

func (collector *resultCollector) runResultProcessor() {
	defer close(collector.processorDoneChan)

	for result := range collector.processorInputChan {
		err := collector.processTransactionResult(
			result.TransactionRequest,
			result.ExecutionSnapshot,
			result.ProcedureOutput,
			result.timeSpent,
			result.numConflictRetries)
		if err != nil {
			collector.processorError = err
			return
		}
	}
}

func (collector *resultCollector) Stop() {
	collector.closeOnce.Do(func() {
		close(collector.processorInputChan)
	})
}

func (collector *resultCollector) Finalize(
	ctx context.Context,
) (
	*execution.ComputationResult,
	error,
) {
	collector.Stop()

	<-collector.processorDoneChan

	if collector.processorError != nil {
		return nil, collector.processorError
	}

	executionDataID, executionDataRoot, err := collector.executionDataProvider.Provide(
		ctx,
		collector.result.Height(),
		collector.result.BlockExecutionData)
	if err != nil {
		return nil, fmt.Errorf("failed to provide execution data: %w", err)
	}

	executionResult := flow.NewExecutionResult(
		collector.parentBlockExecutionResultID,
		collector.result.ExecutableBlock.ID(),
		collector.result.AllChunks(),
		collector.result.AllConvertedServiceEvents(),
		executionDataID)

	executionReceipt, err := GenerateExecutionReceipt(
		collector.signer,
		collector.receiptHasher,
		executionResult,
		collector.spockSignatures)
	if err != nil {
		return nil, fmt.Errorf("could not sign execution result: %w", err)
	}

	collector.result.ExecutionReceipt = executionReceipt
	collector.result.ExecutionDataRoot = executionDataRoot

	collector.metrics.ExecutionBlockExecuted(
		time.Since(collector.blockStartTime),
		collector.blockStats)

	for kind, intensity := range collector.blockMeter.ComputationIntensities() {
		collector.metrics.ExecutionBlockExecutionEffortVectorComponent(
			kind.String(),
			intensity)
	}

	return collector.result, nil
}

func GenerateExecutionReceipt(
	signer module.Local,
	receiptHasher hash.Hasher,
	result *flow.ExecutionResult,
	spockSignatures []crypto.Signature,
) (
	*flow.ExecutionReceipt,
	error,
) {
	receipt := &flow.ExecutionReceipt{
		ExecutionResult:   *result,
		Spocks:            spockSignatures,
		ExecutorSignature: crypto.Signature{},
		ExecutorID:        signer.NodeID(),
	}

	// generates a signature over the execution result
	id := receipt.ID()
	sig, err := signer.Sign(id[:], receiptHasher)
	if err != nil {
		return nil, fmt.Errorf("could not sign execution result: %w", err)
	}

	receipt.ExecutorSignature = sig

	return receipt, nil
}
