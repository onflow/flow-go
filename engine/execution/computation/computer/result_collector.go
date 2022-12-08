package computer

import (
	"sync"

	otelTrace "go.opentelemetry.io/otel/trace"

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

type resultCollector struct {
	tracer    module.Tracer
	blockSpan otelTrace.Span

	metrics module.ExecutionMetrics

	closeOnce sync.Once
	wg        sync.WaitGroup

	committer ViewCommitter
	state     flow.StateCommitment
	viewChan  chan state.View

	eventsListChan chan flow.EventsList

	result *execution.ComputationResult
}

func newResultCollector(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	committer ViewCommitter,
	block *entity.ExecutableBlock,
	numChunks int,
) *resultCollector {
	collector := &resultCollector{
		tracer:         tracer,
		blockSpan:      blockSpan,
		metrics:        metrics,
		committer:      committer,
		state:          *block.StartState,
		viewChan:       make(chan state.View, numChunks),
		eventsListChan: make(chan flow.EventsList, numChunks),
		result:         execution.NewEmptyComputationResult(block),
	}

	collector.wg.Add(2)
	go collector.runChunkCommitter()
	go collector.runChunkHasher()

	return collector
}

func (collector *resultCollector) runChunkCommitter() {
	defer collector.wg.Done()

	for view := range collector.viewChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXECommitDelta)

		stateCommit, proof, trieUpdate, err := collector.committer.CommitView(
			view,
			collector.state)
		if err != nil {
			panic(err)
		}

		collector.result.StateCommitments = append(
			collector.result.StateCommitments,
			stateCommit)
		collector.result.Proofs = append(collector.result.Proofs, proof)
		collector.result.TrieUpdates = append(
			collector.result.TrieUpdates,
			trieUpdate)

		collector.state = stateCommit
		span.End()
	}
}

func (collector *resultCollector) runChunkHasher() {
	defer collector.wg.Done()

	for data := range collector.eventsListChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXEHashEvents)

		rootHash, err := flow.EventsMerkleRootHash(data)
		if err != nil {
			panic(err)
		}

		collector.result.EventsHashes = append(
			collector.result.EventsHashes,
			rootHash)

		span.End()
	}
}

func (collector *resultCollector) AddTransactionResult(
	collectionIndex int,
	txn *fvm.TransactionProcedure,
) {
	collector.result.AddTransactionResult(collectionIndex, txn)
}

func (collector *resultCollector) CommitCollection(
	collectionIndex int,
	collection collectionItem,
	collectionView state.View,
) module.ExecutionResultStats {
	collector.viewChan <- collectionView
	collector.eventsListChan <- collector.result.Events[collectionIndex]

	collector.result.AddCollection(collectionView.(*delta.View).Interactions())
	return collector.result.CollectionStats(collectionIndex)
}

func (collector *resultCollector) Stop() {
	collector.closeOnce.Do(func() {
		close(collector.viewChan)
		close(collector.eventsListChan)
	})
}

func (collector *resultCollector) Finalize() *execution.ComputationResult {
	collector.Stop()
	collector.wg.Wait()
	return collector.result
}
