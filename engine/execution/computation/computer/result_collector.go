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
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/trace"
)

// ViewCommitter commits views's deltas to the ledger and collects the proofs
type ViewCommitter interface {
	// CommitView commits a views' register delta and collects proofs
	CommitView(
		*state.ExecutionSnapshot,
		flow.StateCommitment,
	) (
		flow.StateCommitment,
		[]byte,
		*ledger.TrieUpdate,
		error,
	)
}

type transactionResult struct {
	transaction
	*state.ExecutionSnapshot
	fvm.ProcedureOutput
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

	executionDataProvider *provider.Provider

	parentBlockExecutionResultID flow.Identifier

	result    *execution.ComputationResult
	consumers []result.ExecutedCollectionConsumer

	chunks                 []*flow.Chunk
	spockSignatures        []crypto.Signature
	convertedServiceEvents flow.ServiceEventList

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
	committer ViewCommitter,
	signer module.Local,
	executionDataProvider *provider.Provider,
	spockHasher hash.Hasher,
	receiptHasher hash.Hasher,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	numTransactions int,
	consumers []result.ExecutedCollectionConsumer,
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
		chunks:                       make([]*flow.Chunk, 0, numCollections),
		spockSignatures:              make([]crypto.Signature, 0, numCollections),
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
	defer collector.tracer.StartSpanFromParent(
		collector.blockSpan,
		trace.EXECommitDelta).End()

	startState := collector.result.EndState
	endState, proof, trieUpdate, err := collector.committer.CommitView(
		collectionExecutionSnapshot,
		startState)
	if err != nil {
		return fmt.Errorf("commit view failed: %w", err)
	}

	events := collector.result.Events[collection.collectionIndex]
	eventsHash, err := flow.EventsMerkleRootHash(events)
	if err != nil {
		return fmt.Errorf("hash events failed: %w", err)
	}

	collector.result.EventsHashes = append(
		collector.result.EventsHashes,
		eventsHash)

	chunk := flow.NewChunk(
		collection.blockId,
		collection.collectionIndex,
		startState,
		len(collection.Transactions),
		eventsHash,
		endState)
	collector.chunks = append(collector.chunks, chunk)

	collectionStruct := collection.Collection()

	// Note: There's some inconsistency in how chunk execution data and
	// chunk data pack populate their collection fields when the collection
	// is the system collection.
	executionCollection := &collectionStruct
	dataPackCollection := executionCollection
	if collection.isSystemTransaction {
		dataPackCollection = nil
	}

	collector.result.ChunkDataPacks = append(
		collector.result.ChunkDataPacks,
		flow.NewChunkDataPack(
			chunk.ID(),
			startState,
			proof,
			dataPackCollection))

	collector.result.ChunkExecutionDatas = append(
		collector.result.ChunkExecutionDatas,
		&execution_data.ChunkExecutionData{
			Collection: executionCollection,
			Events:     collector.result.Events[collection.collectionIndex],
			TrieUpdate: trieUpdate,
		})

	collector.metrics.ExecutionChunkDataPackGenerated(
		len(proof),
		len(collection.Transactions))

	collector.result.EndState = endState

	collector.result.TransactionResultIndex = append(
		collector.result.TransactionResultIndex,
		len(collector.result.TransactionResults))
	collector.result.StateSnapshots = append(
		collector.result.StateSnapshots,
		collectionExecutionSnapshot)

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

	collector.currentCollectionStartTime = time.Now()
	collector.currentCollectionView = delta.NewDeltaView(nil)
	collector.currentCollectionStats = module.ExecutionResultStats{
		NumberOfCollections: 1,
	}

	for _, consumer := range collector.consumers {
		err = consumer.OnExecutedCollection(collector.result.CollectionResult(collection.collectionIndex))
		if err != nil {
			return fmt.Errorf("consumer failed: %w", err)
		}
	}

	return nil
}

func (collector *resultCollector) processTransactionResult(
	txn transaction,
	txnExecutionSnapshot *state.ExecutionSnapshot,
	output fvm.ProcedureOutput,
) error {
	collector.convertedServiceEvents = append(
		collector.convertedServiceEvents,
		output.ConvertedServiceEvents...)

	collector.result.Events[txn.collectionIndex] = append(
		collector.result.Events[txn.collectionIndex],
		output.Events...)
	collector.result.ServiceEvents = append(
		collector.result.ServiceEvents,
		output.ServiceEvents...)

	txnResult := flow.TransactionResult{
		TransactionID:   txn.ID,
		ComputationUsed: output.ComputationUsed,
		MemoryUsed:      output.MemoryEstimate,
	}
	if output.Err != nil {
		txnResult.ErrorMessage = output.Err.Error()
	}

	collector.result.TransactionResults = append(
		collector.result.TransactionResults,
		txnResult)

	for computationKind, intensity := range output.ComputationIntensities {
		collector.result.ComputationIntensities[computationKind] += intensity
	}

	err := collector.currentCollectionView.Merge(txnExecutionSnapshot)
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
		collector.currentCollectionView.Finalize())
}

func (collector *resultCollector) AddTransactionResult(
	txn transaction,
	snapshot *state.ExecutionSnapshot,
	output fvm.ProcedureOutput,
) {
	result := transactionResult{
		transaction:       txn,
		ExecutionSnapshot: snapshot,
		ProcedureOutput:   output,
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
			result.ExecutionSnapshot,
			result.ProcedureOutput)
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

	executionDataID, err := collector.executionDataProvider.Provide(
		ctx,
		collector.result.Height(),
		collector.result.BlockExecutionData)
	if err != nil {
		return nil, fmt.Errorf("failed to provide execution data: %w", err)
	}

	executionResult := flow.NewExecutionResult(
		collector.parentBlockExecutionResultID,
		collector.result.ExecutableBlock.ID(),
		collector.chunks,
		collector.convertedServiceEvents,
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

	collector.metrics.ExecutionBlockExecuted(
		time.Since(collector.blockStartTime),
		collector.blockStats)

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
