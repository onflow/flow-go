package computer

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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

	signer                  module.Local
	spockHasher             hash.Hasher
	snapshotHasherInputChan chan collectionResult
	snapshotHasherDoneChan  chan struct{}
	snapshotHasherError     error

	result *execution.ComputationResult
}

func newResultCollector(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	committer ViewCommitter,
	signer module.Local,
	spockHasher hash.Hasher,
	block *entity.ExecutableBlock,
	numCollections int,
) *resultCollector {
	collector := &resultCollector{
		tracer:                  tracer,
		blockSpan:               blockSpan,
		metrics:                 metrics,
		committer:               committer,
		committerInputChan:      make(chan collectionResult, numCollections),
		committerDoneChan:       make(chan struct{}),
		signer:                  signer,
		spockHasher:             spockHasher,
		snapshotHasherInputChan: make(chan collectionResult, numCollections),
		snapshotHasherDoneChan:  make(chan struct{}),
		result:                  execution.NewEmptyComputationResult(block),
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
		collector.result.TrieUpdates = append(
			collector.result.TrieUpdates,
			trieUpdate)

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
		collector.result.Chunks = append(collector.result.Chunks, chunk)

		var flowCollection *flow.Collection
		if !collection.isSystemCollection {
			collectionStruct := collection.CompleteCollection.Collection()
			flowCollection = &collectionStruct
		}

		collector.result.ChunkDataPacks = append(
			collector.result.ChunkDataPacks,
			flow.NewChunkDataPack(
				chunk.ID(),
				startState,
				proof,
				flowCollection))

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

		collector.result.SpockSignatures = append(
			collector.result.SpockSignatures,
			spock)
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
func (collector *resultCollector) Finalize() (
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

	return collector.result, nil
}
