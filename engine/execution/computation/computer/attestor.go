package computer

import (
	"fmt"

	otelTrace "go.opentelemetry.io/otel/trace"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/result"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
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

type attestor struct {
	tracer    module.Tracer
	blockSpan otelTrace.Span
	metrics   module.ExecutionMetrics
	committer ViewCommitter

	result    *execution.AttestationResults
	consumers []result.AttestedCollectionConsumer
}

func newAttestor(
	tracer module.Tracer,
	blockSpan otelTrace.Span,
	metrics module.ExecutionMetrics,
	committer ViewCommitter,
	block *entity.ExecutableBlock,
	consumers []result.AttestedCollectionConsumer,
) *attestor {
	return &attestor{
		tracer:    tracer,
		blockSpan: blockSpan,
		metrics:   metrics,
		committer: committer,
		result:    execution.NewEmptyAttestationResult(block),
		consumers: consumers,
	}
}

func (at *attestor) OnExecutedCollection(ec result.ExecutedCollection) error {

	defer at.tracer.StartSpanFromParent(
		at.blockSpan,
		trace.EXECommitDelta).End()

	startState := at.result.EndState
	endState, proof, trieUpdate, err := at.committer.CommitView(
		ec.ExecutionSnapshot(),
		startState)
	if err != nil {
		return fmt.Errorf("commit view failed: %w", err)
	}

	eventsHash, err := flow.EventsMerkleRootHash(ec.EmittedEvents())
	if err != nil {
		return fmt.Errorf("hash events failed: %w", err)
	}

	at.result.EventsHashes = append(
		at.result.EventsHashes,
		eventsHash)

	executionCollection := ec.Collection()

	chunk := flow.NewChunk(
		ec.BlockHeader().ID(),
		ec.CollectionIndex(),
		startState,
		len(executionCollection.Transactions),
		eventsHash,
		endState)
	at.result.Chunks = append(at.result.Chunks, chunk)

	dataPackCollection := executionCollection
	if ec.IsSystemCollection() {
		dataPackCollection = nil
	}

	at.result.ChunkDataPacks = append(
		at.result.ChunkDataPacks,
		flow.NewChunkDataPack(
			chunk.ID(),
			startState,
			proof,
			dataPackCollection))

	at.result.ChunkExecutionDatas = append(
		at.result.ChunkExecutionDatas,
		&execution_data.ChunkExecutionData{
			Collection: executionCollection,
			Events:     ec.EmittedEvents(),
			TrieUpdate: trieUpdate,
		})

	at.metrics.ExecutionChunkDataPackGenerated(
		len(proof),
		len(executionCollection.Transactions))

	at.result.EndState = endState

	for _, consumer := range at.consumers {
		err = consumer.OnAttestedCollection(at.result.AttestedCollection(ec.CollectionIndex()))
		if err != nil {
			return fmt.Errorf("consumer failed: %w", err)
		}
	}

	return nil
}
