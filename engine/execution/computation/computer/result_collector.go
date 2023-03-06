package computer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution"
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
		state.View,
		flow.StateCommitment,
	) (
		flow.StateCommitment,
		[]byte,
		*ledger.TrieUpdate,
		error,
	)
}

type collectionResult struct {
	collectionItem
	startTime time.Time
	state.View
}

type resultCollector struct {
	tracer    module.Tracer
	blockSpan otelTrace.Span

	metrics module.ExecutionMetrics

	closeOnce sync.Once

	committer          ViewCommitter
	committerInputChan chan collectionResult
	committerDoneChan  chan struct{}
	committerError     error

	signer        module.Local
	spockHasher   hash.Hasher
	receiptHasher hash.Hasher

	snapshotHasherInputChan chan collectionResult
	snapshotHasherDoneChan  chan struct{}
	snapshotHasherError     error

	executionDataProvider *provider.Provider

	parentBlockExecutionResultID flow.Identifier

	result *execution.ComputationResult

	chunks          []*flow.Chunk
	spockSignatures []crypto.Signature
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
	numCollections int,
) *resultCollector {
	collector := &resultCollector{
		tracer:                       tracer,
		blockSpan:                    blockSpan,
		metrics:                      metrics,
		committer:                    committer,
		committerInputChan:           make(chan collectionResult, numCollections),
		committerDoneChan:            make(chan struct{}),
		signer:                       signer,
		spockHasher:                  spockHasher,
		receiptHasher:                receiptHasher,
		snapshotHasherInputChan:      make(chan collectionResult, numCollections),
		snapshotHasherDoneChan:       make(chan struct{}),
		executionDataProvider:        executionDataProvider,
		parentBlockExecutionResultID: parentBlockExecutionResultID,
		result:                       execution.NewEmptyComputationResult(block),
		chunks:                       make([]*flow.Chunk, 0, numCollections),
		spockSignatures:              make([]crypto.Signature, 0, numCollections),
	}

	go collector.runCollectionCommitter()
	go collector.runSnapshotHasher()

	return collector
}

func (collector *resultCollector) runCollectionCommitter() {
	defer close(collector.committerDoneChan)

	for collection := range collector.committerInputChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXECommitDelta)

		startState := collector.result.EndState
		endState, proof, trieUpdate, err := collector.committer.CommitView(
			collection.View,
			startState)
		if err != nil {
			collector.committerError = fmt.Errorf(
				"commit view failed: %w",
				err)
			return
		}

		collector.result.StateCommitments = append(
			collector.result.StateCommitments,
			endState)
		collector.result.Proofs = append(collector.result.Proofs, proof)

		eventsHash, err := flow.EventsMerkleRootHash(
			collector.result.Events[collection.collectionIndex])
		if err != nil {
			collector.committerError = fmt.Errorf(
				"hash events failed: %w",
				err)
			return
		}

		collector.result.EventsHashes = append(
			collector.result.EventsHashes,
			eventsHash)

		chunk := flow.NewChunk(
			collection.blockId,
			collection.collectionIndex,
			startState,
			len(collection.transactions),
			eventsHash,
			endState)
		collector.chunks = append(collector.chunks, chunk)

		collectionStruct := collection.Collection()

		// Note: There's some inconsistency in how chunk execution data and
		// chunk data pack populate their collection fields when the collection
		// is the system collection.
		executionCollection := &collectionStruct
		dataPackCollection := executionCollection
		if collection.isSystemCollection {
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
			len(collection.transactions))

		collector.result.EndState = endState

		span.End()
	}
}

func (collector *resultCollector) runSnapshotHasher() {
	defer close(collector.snapshotHasherDoneChan)

	for collection := range collector.snapshotHasherInputChan {

		snapshot := collection.View.(*delta.View).Interactions()
		collector.result.AddCollection(snapshot)

		collector.metrics.ExecutionCollectionExecuted(
			time.Since(collection.startTime),
			collector.result.CollectionStats(collection.collectionIndex))

		spock, err := collector.signer.SignFunc(
			snapshot.SpockSecret,
			collector.spockHasher,
			SPOCKProve)
		if err != nil {
			collector.snapshotHasherError = fmt.Errorf(
				"signing spock hash failed: %w",
				err)
			return
		}

		collector.spockSignatures = append(collector.spockSignatures, spock)
	}
}

func (collector *resultCollector) AddTransactionResult(
	collectionIndex int,
	txn *fvm.TransactionProcedure,
) {
	collector.result.AddTransactionResult(collectionIndex, txn)
}

func (collector *resultCollector) CommitCollection(
	collection collectionItem,
	startTime time.Time,
	collectionView state.View,
) {

	result := collectionResult{
		collectionItem: collection,
		startTime:      startTime,
		View:           collectionView,
	}

	select {
	case collector.committerInputChan <- result:
		// Do nothing
	case <-collector.committerDoneChan:
		// Committer exited (probably due to an error)
	}

	select {
	case collector.snapshotHasherInputChan <- result:
		// do nothing
	case <-collector.snapshotHasherDoneChan:
		// Snapshot hasher exited (probably due to an error)
	}
}

func (collector *resultCollector) Stop() {
	collector.closeOnce.Do(func() {
		close(collector.committerInputChan)
		close(collector.snapshotHasherInputChan)
	})
}

// TODO(patrick): refactor execution receipt generation from ingress engine
// to here to improve benchmarking.
func (collector *resultCollector) Finalize(
	ctx context.Context,
) (
	*execution.ComputationResult,
	error,
) {
	collector.Stop()

	<-collector.committerDoneChan
	<-collector.snapshotHasherDoneChan

	var err error
	if collector.committerError != nil {
		err = multierror.Append(err, collector.committerError)
	}

	if collector.snapshotHasherError != nil {
		err = multierror.Append(err, collector.snapshotHasherError)
	}

	if err != nil {
		return nil, err
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
		collector.result.ConvertedServiceEvents,
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
