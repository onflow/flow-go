package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/result"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
)

type transactionResult struct {
	transaction
	*state.ExecutionSnapshot
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

	signer        module.Local
	spockHasher   hash.Hasher
	receiptHasher hash.Hasher

	executionDataProvider *provider.Provider

	parentBlockExecutionResultID flow.Identifier

	result    *execution.ComputationResult
	consumers []result.ExecutedCollectionConsumer

	blockStartTime time.Time
	blockStats     module.ExecutionResultStats

	currentCollectionStartTime time.Time
	currentCollectionView      *delta.View
	currentCollectionStats     module.ExecutionResultStats
}

func newResultCollector(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	signer module.Local,
	executionDataProvider *provider.Provider,
	spockHasher hash.Hasher,
	receiptHasher hash.Hasher,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	numTransactions int,
	consumers []result.ExecutedCollectionConsumer,
) *resultCollector {
	now := time.Now()
	collector := &resultCollector{
		tracer:                       tracer,
		blockSpan:                    blockSpan,
		metrics:                      metrics,
		processorInputChan:           make(chan transactionResult, numTransactions),
		processorDoneChan:            make(chan struct{}),
		signer:                       signer,
		spockHasher:                  spockHasher,
		receiptHasher:                receiptHasher,
		executionDataProvider:        executionDataProvider,
		parentBlockExecutionResultID: parentBlockExecutionResultID,
		result:                       execution.NewEmptyComputationResult(block),
		consumers:                    consumers,
		blockStartTime:               now,
		currentCollectionStartTime:   now,
		currentCollectionView:        delta.NewDeltaView(nil),
		currentCollectionStats: module.ExecutionResultStats{
			NumberOfCollections: 1,
		},
	}

	go collector.runResultProcessor()

	return collector
}

func (collector *resultCollector) commitCollection(
	collection collectionInfo,
	startTime time.Time,
	collectionExecutionSnapshot *state.ExecutionSnapshot,
) error {
	collector.result.TransactionResultIndex = append(
		collector.result.TransactionResultIndex,
		len(collector.result.TransactionResults))

	collector.result.StateSnapshots = append(
		collector.result.StateSnapshots,
		collectionExecutionSnapshot)

	events := collector.result.Events[collection.collectionIndex]
	collector.currentCollectionStats.EventCounts = len(events)
	collector.currentCollectionStats.EventSize = events.ByteSize()
	collector.currentCollectionStats.NumberOfRegistersTouched = len(
		collectionExecutionSnapshot.AllRegisterIDs())
	for _, entry := range collectionExecutionSnapshot.UpdatedRegisters() {
		collector.currentCollectionStats.NumberOfBytesWrittenToRegisters += len(
			entry.Value)
	}

	for _, consumer := range collector.consumers {
		err := consumer.OnExecutedCollection(collector.result.CollectionResult(collection.collectionIndex))
		if err != nil {
			return fmt.Errorf("consumer failed: %w", err)
		}
	}

	collector.metrics.ExecutionCollectionExecuted(
		time.Since(startTime),
		collector.currentCollectionStats)

	collector.blockStats.Merge(collector.currentCollectionStats)

	collector.currentCollectionStartTime = time.Now()
	collector.currentCollectionView = delta.NewDeltaView(nil)
	collector.currentCollectionStats = module.ExecutionResultStats{
		NumberOfCollections: 1,
	}

	return nil
}

func (collector *resultCollector) processTransactionResult(
	txn transaction,
	txnExecutionSnapshot *state.ExecutionSnapshot,
) error {
	collector.convertedServiceEvents = append(
		collector.convertedServiceEvents,
		txn.ConvertedServiceEvents...)

	collector.result.Events[txn.collectionIndex] = append(
		collector.result.Events[txn.collectionIndex],
		txn.Events...)
	collector.result.ServiceEvents = append(
		collector.result.ServiceEvents,
		txn.ServiceEvents...)

	txnResult := flow.TransactionResult{
		TransactionID:   txn.ID,
		ComputationUsed: txn.ComputationUsed,
		MemoryUsed:      txn.MemoryEstimate,
	}
	if txn.Err != nil {
		txnResult.ErrorMessage = txn.Err.Error()
	}

	collector.result.TransactionResults = append(
		collector.result.TransactionResults,
		txnResult)

	for computationKind, intensity := range txn.ComputationIntensities {
		collector.result.ComputationIntensities[computationKind] += intensity
	}

	err := collector.currentCollectionView.Merge(txnExecutionSnapshot)
	if err != nil {
		return fmt.Errorf("failed to merge into collection view: %w", err)
	}

	collector.currentCollectionStats.ComputationUsed += txn.ComputationUsed
	collector.currentCollectionStats.MemoryUsed += txn.MemoryEstimate
	collector.currentCollectionStats.NumberOfTransactions += 1

	if !txn.lastTransactionInCollection {
		return nil
	}

	return collector.commitCollection(
		txn.collectionInfo,
		collector.currentCollectionStartTime,
		collector.currentCollectionView.Finalize())
}

func (collector *resultCollector) AddTransactionResult(
	txn transaction,
	snapshot *state.ExecutionSnapshot,
) {
	result := transactionResult{
		transaction:       txn,
		ExecutionSnapshot: snapshot,
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
			result.transaction,
			result.ExecutionSnapshot)
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

	collector.metrics.ExecutionBlockExecuted(
		time.Since(collector.blockStartTime),
		collector.blockStats)

	return collector.result, nil
}
