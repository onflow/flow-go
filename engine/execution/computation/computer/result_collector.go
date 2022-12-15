package computer

import (
	"fmt"
	"sync"

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

	eventHasherInputChan chan flow.EventsList
	eventHasherDoneChan  chan struct{}
	eventHasherError     error

	signer               module.Local
	spockHasher          hash.Hasher
	spockHasherInputChan chan *delta.SpockSnapshot
	spockHasherDoneChan  chan struct{}
	spockHasherError     error

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
		tracer:               tracer,
		blockSpan:            blockSpan,
		metrics:              metrics,
		committer:            committer,
		state:                *block.StartState,
		committerInputChan:   make(chan state.View, numCollections),
		committerDoneChan:    make(chan struct{}),
		eventHasherInputChan: make(chan flow.EventsList, numCollections),
		eventHasherDoneChan:  make(chan struct{}),
		signer:               signer,
		spockHasher:          spockHasher,
		spockHasherInputChan: make(chan *delta.SpockSnapshot, numCollections),
		spockHasherDoneChan:  make(chan struct{}),
		result:               execution.NewEmptyComputationResult(block),
	}

	go collector.runCollectionCommitter()
	go collector.runEventsHasher()
	go collector.runSpockHasher()

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

func (collector *resultCollector) runEventsHasher() {
	defer close(collector.eventHasherDoneChan)

	for data := range collector.eventHasherInputChan {
		span := collector.tracer.StartSpanFromParent(
			collector.blockSpan,
			trace.EXEHashEvents)

		rootHash, err := flow.EventsMerkleRootHash(data)
		if err != nil {
			collector.eventHasherError = fmt.Errorf(
				"event hasher failed: %w",
				err)
			return
		}

		collector.result.EventsHashes = append(
			collector.result.EventsHashes,
			rootHash)

		span.End()
	}
}

func (collector *resultCollector) runSpockHasher() {
	defer close(collector.spockHasherDoneChan)

	for snapshot := range collector.spockHasherInputChan {
		spock, err := collector.signer.SignFunc(
			snapshot.SpockSecret,
			collector.spockHasher,
			spockProveWrapper)
		if err != nil {
			collector.spockHasherError = fmt.Errorf(
				"spock hasher failed: %w",
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
	case collector.eventHasherInputChan <- collector.result.Events[collectionIndex]:
		// Do nothing
	case <-collector.eventHasherDoneChan:
		// Events hasher exited (probably due to an error)
	}

	snapshot := collectionView.(*delta.View).Interactions()
	select {
	case collector.spockHasherInputChan <- snapshot:
		// do nothing
	case <-collector.spockHasherDoneChan:
		// Spock hasher exited (probably due to an error)
	}

	collector.result.AddCollection(snapshot)
	return collector.result.CollectionStats(collectionIndex)
}

func (collector *resultCollector) Stop() {
	collector.closeOnce.Do(func() {
		close(collector.committerInputChan)
		close(collector.eventHasherInputChan)
		close(collector.spockHasherInputChan)
	})
}

func (collector *resultCollector) Finalize() (
	*execution.ComputationResult,
	error,
) {
	collector.Stop()

	<-collector.committerDoneChan
	<-collector.eventHasherDoneChan
	<-collector.spockHasherDoneChan

	var err error
	if collector.committerError != nil {
		err = multierror.Append(err, collector.committerError)
	}

	if collector.eventHasherError != nil {
		err = multierror.Append(err, collector.eventHasherError)
	}

	if collector.spockHasherError != nil {
		err = multierror.Append(err, collector.spockHasherError)
	}

	if err != nil {
		return nil, err
	}

	return collector.result, nil
}
