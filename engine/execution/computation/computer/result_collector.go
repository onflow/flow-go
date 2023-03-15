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
	"github.com/onflow/flow-go/engine/execution/state/delta"
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
		state.View,
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
	state.ExecutionSnapshot
}

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

	result *execution.ComputationResult

	chunks                 []*flow.Chunk
	spockSignatures        []crypto.Signature
	convertedServiceEvents flow.ServiceEventList

	currentCollectionStartTime time.Time
	currentCollectionView      *delta.View
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
) *resultCollector {
	numCollections := len(block.Collections()) + 1
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
		chunks:                       make([]*flow.Chunk, 0, numCollections),
		spockSignatures:              make([]crypto.Signature, 0, numCollections),
		currentCollectionStartTime:   time.Now(),
		currentCollectionView:        delta.NewDeltaView(nil),
	}

	go collector.runResultProcessor()

	return collector
}

func (collector *resultCollector) commitCollection(
	collection collectionInfo,
	startTime time.Time,
	// TODO(patrick): switch to ExecutionSnapshot
	collectionExecutionSnapshot state.View,
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

	collector.result.StateCommitments = append(
		collector.result.StateCommitments,
		endState)

	eventsHash, err := flow.EventsMerkleRootHash(
		collector.result.Events[collection.collectionIndex])
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

	return nil
}

func (collector *resultCollector) hashCollection(
	collection collectionInfo,
	startTime time.Time,
	collectionExecutionSnapshot state.ExecutionSnapshot,
) error {
	collector.result.TransactionResultIndex = append(
		collector.result.TransactionResultIndex,
		len(collector.result.TransactionResults))
	collector.result.StateSnapshots = append(
		collector.result.StateSnapshots,
		collectionExecutionSnapshot)

	collector.metrics.ExecutionCollectionExecuted(
		time.Since(startTime),
		collector.result.CollectionStats(collection.collectionIndex))

	spock, err := collector.signer.SignFunc(
		collectionExecutionSnapshot.SpockSecret(),
		collector.spockHasher,
		SPOCKProve)
	if err != nil {
		return fmt.Errorf("signing spock hash failed: %w", err)
	}

	collector.spockSignatures = append(collector.spockSignatures, spock)

	return nil
}

func (collector *resultCollector) processTransactionResult(
	txn transaction,
	txnExecutionSnapshot state.ExecutionSnapshot,
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

	if !txn.lastTransactionInCollection {
		return nil
	}

	err = collector.commitCollection(
		txn.collectionInfo,
		collector.currentCollectionStartTime,
		collector.currentCollectionView)
	if err != nil {
		return err
	}

	err = collector.hashCollection(
		txn.collectionInfo,
		collector.currentCollectionStartTime,
		collector.currentCollectionView)
	if err != nil {
		return err
	}

	collector.currentCollectionStartTime = time.Now()
	collector.currentCollectionView = delta.NewDeltaView(nil)

	return nil
}

func (collector *resultCollector) AddTransactionResult(
	txn transaction,
	snapshot state.ExecutionSnapshot,
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
