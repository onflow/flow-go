package assigner

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// The Assigner engine reads the receipts from each finalized block.
// For each receipt, it reads its result and find the chunks the assigned
// to me to verify, and then save it to the chunks job queue for the
// fetcher engine to process.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	metrics module.VerificationMetrics
	me      module.Local
	state   protocol.State

	headerStorage    storage.Headers       // used to check block existence before verifying
	assigner         module.ChunkAssigner  // used to determine chunks this node needs to verify
	chunksQueue      storage.ChunksQueue   // to store chunks to be verified
	newChunkListener module.NewJobListener // to notify about a new chunk
	finishProcessing finishProcessing      // to report a block has been processed
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	me module.Local,
	state protocol.State,
	headerStorage storage.Headers,
	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
) *Engine {
	e := &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "assigner").Logger(),
		metrics:          metrics,
		me:               me,
		state:            state,
		headerStorage:    headerStorage,
		assigner:         assigner,
		chunksQueue:      chunksQueue,
		newChunkListener: newChunkListener,
	}

	return e
}

func (e *Engine) handleExecutionReceipt(receipt *flow.ExecutionReceipt, containerBlockID flow.Identifier) {

}

func (e *Engine) withFinishProcessing(finishProcessing finishProcessing) {
	e.finishProcessing = finishProcessing
}

func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	return nil
}

// ProcessFinalizedBlock process each finalized block, and find the chunks that
// assigned to me, and store it to the chunks job queue.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	// the block consumer will pull as many finalized blocks as
	// it can consume to process
	blockID := block.ID()
	for _, receipt := range block.Payload.Receipts {
		e.handleExecutionReceipt(receipt, blockID)
	}
}

// myChunkAssignments returns the list of chunks in the chunk list assigned to this verification node.
func (e *Engine) myChunkAssignments(ctx context.Context, result *flow.ExecutionResult) (flow.ChunkList, error) {
	var span opentracing.Span
	span, _ = e.tracer.StartSpanFromContext(ctx, trace.VERMatchMyChunkAssignments)
	defer span.Finish()

	assignment, err := e.assigner.Assign(result, result.BlockID)
	if err != nil {
		return nil, err
	}

	mine, err := myChunks(e.me.NodeID(), assignment, result.Chunks)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

func myChunks(myID flow.Identifier, assignment *chunks.Assignment, chunks flow.ChunkList) (flow.ChunkList, error) {
	// indices of chunks assigned to verifier
	chunkIndices := assignment.ByNodeID(myID)

	// chunks keeps the list of chunks assigned to the verifier
	myChunks := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		chunk, ok := chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}

		myChunks = append(myChunks, chunk)
	}

	return myChunks, nil
}
