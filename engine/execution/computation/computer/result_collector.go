package computer

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
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

	committer          ViewCommitter
	state              flow.StateCommitment
	committerInputChan chan state.View
	committerDoneChan  chan struct{}
	committerError     error

	hasherInputChan chan flow.EventsList
	hasherDoneChan  chan struct{}
	hasherError     error

	result *execution.ComputationResult
}

func newResultCollector(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	committer ViewCommitter,
	block *entity.ExecutableBlock,
	numCollections int,
) *resultCollector {
	collector := &resultCollector{
		tracer:             tracer,
		blockSpan:          blockSpan,
		metrics:            metrics,
		committer:          committer,
		state:              *block.StartState,
		committerInputChan: make(chan state.View, numCollections),
		committerDoneChan:  make(chan struct{}),
		hasherInputChan:    make(chan flow.EventsList, numCollections),
		hasherDoneChan:     make(chan struct{}),
		result:             execution.NewEmptyComputationResult(block),
	}

	go collector.runCollectionCommitter()
	go collector.runCollectionHasher()

	return collector
}

func (collector *resultCollector) runCollectionCommitter() {
	defer close(collector.committerDoneChan)

	for view := range collector.committerInputChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXECommitDelta)

		stateCommit, proof, trieUpdate, err := collector.committer.CommitView(
			view,
			collector.state)
		if err != nil {
			collector.committerError = fmt.Errorf("committer failed: %w", err)
			return
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

func (collector *resultCollector) runCollectionHasher() {
	defer close(collector.hasherDoneChan)

	for data := range collector.hasherInputChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXEHashEvents)

		rootHash, err := flow.EventsMerkleRootHash(data)
		if err != nil {
			collector.hasherError = fmt.Errorf("hasher failed: %w", err)
			return
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

	select {
	case collector.committerInputChan <- collectionView:
		// Do nothing
	case <-collector.committerDoneChan:
		// Committer exited (probably due to an error)
	}

	select {
	case collector.hasherInputChan <- collector.result.Events[collectionIndex]:
		// Do nothing
	case <-collector.hasherDoneChan:
		// Hasher exited (probably due to an error)
	}

	collector.result.AddCollection(collectionView.(*delta.View).Interactions())
	return collector.result.CollectionStats(collectionIndex)
}

func (collector *resultCollector) Stop() {
	collector.closeOnce.Do(func() {
		close(collector.committerInputChan)
		close(collector.hasherInputChan)
	})
}

func (collector *resultCollector) Finalize() (
	*execution.ComputationResult,
	error,
) {
	collector.Stop()

	<-collector.committerDoneChan
	<-collector.hasherDoneChan

	var err error
	if collector.committerError != nil {
		err = multierror.Append(err, collector.committerError)
	}

	if collector.hasherError != nil {
		err = multierror.Append(err, collector.hasherError)
	}

	if err != nil {
		return nil, err
	}

	return collector.result, nil
}
