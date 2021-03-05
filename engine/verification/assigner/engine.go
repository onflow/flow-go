package assigner

import (
	"context"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// The Assigner engine reads the receipts from each finalized block.
// For each receipt, it reads its result and find the chunks the assigned
// to me to verify, and then save it to the chunks job queue for the
// fetcher engine to process.
type Engine struct {
	unit                  *engine.Unit
	log                   zerolog.Logger
	metrics               module.VerificationMetrics
	tracer                module.Tracer
	me                    module.Local
	state                 protocol.State
	assigner              module.ChunkAssigner      // to determine chunks this node should verify.
	chunksQueue           storage.ChunksQueue       // to store chunks to be verified.
	newChunkListener      module.NewJobListener     // to notify chunk queue consumer about a new chunk.
	blockConsumerNotifier module.ProcessingNotifier // to report a block has been processed.
	indexer               Indexer                   // to index receipts of a block based on their executor identifier.
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	me module.Local,
	state protocol.State,
	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
	indexer Indexer,
) *Engine {
	return &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "assigner").Logger(),
		metrics:          metrics,
		tracer:           tracer,
		me:               me,
		state:            state,
		assigner:         assigner,
		chunksQueue:      chunksQueue,
		newChunkListener: newChunkListener,
		indexer:          indexer,
	}
}

func (e *Engine) withBlockConsumerNotifier(notifier module.ProcessingNotifier) {
	e.blockConsumerNotifier = notifier
}

func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// receiptChunkAssignment receives a receipt that appears in a finalized container block. In case this verification node
// is staked at the reference block of this execution receipt's result, chunk assignment is done on the execution result of the receipt,
// and the list of assigned chunks returned.
func (e *Engine) receiptChunkAssignment(ctx context.Context,
	receipt *flow.ExecutionReceipt,
	containerBlockID flow.Identifier) (flow.ChunkList, error) {
	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()
	referenceBlockID := receipt.ExecutionResult.BlockID

	log := log.With().
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).
		Hex("reference_block_id", logging.ID(referenceBlockID)).
		Hex("container_block_id", logging.ID(containerBlockID)).
		Logger()

	e.metrics.OnExecutionReceiptReceived()

	// verification node should be staked at the reference block id.
	ok, err := stakedAsVerification(e.state, referenceBlockID, e.me.NodeID())
	if err != nil {
		return nil, fmt.Errorf("could not verify stake of verification node for result at reference block id: %w", err)
	}
	if !ok {
		log.Warn().Msg("node is not staked at reference block id, receipt is discarded")
		return nil, nil
	}

	// chunk assignment
	chunkList, err := e.chunkAssignments(ctx, &receipt.ExecutionResult)
	if err != nil {
		return nil, fmt.Errorf("could not determine chunk assignment: %w", err)
	}
	e.metrics.OnChunksAssigned(len(chunkList))

	// TODO: de-escalate to debug level on stable version.
	log.Info().
		Int("total_assigned_chunks", len(chunkList)).
		Msg("chunk assignment done")

	return chunkList, nil
}

// processChunk receives a chunk that belongs to execution result id. It creates a chunk locator
// for the chunk and stores the chunk locator in the chunks queue.
//
// Note that the chunk is assume to be legitimately assigned to this verification node
// (through the chunk assigner), and belong to the execution result.
//
// Deduplication of chunk locators is delegated to the chunks queue.
func (e *Engine) processChunk(chunk *flow.Chunk, resultID flow.Identifier) error {
	log := e.log.With().
		Hex("result_id", logging.ID(resultID)).
		Hex("chunk_id", logging.ID(chunk.ID())).
		Uint64("chunk_index", chunk.Index).Logger()

	locator := &chunks.Locator{
		ResultID: resultID,
		Index:    chunk.Index,
	}

	// pushes chunk locator to the chunks queue
	ok, err := e.chunksQueue.StoreChunkLocator(locator)
	if err != nil {
		return fmt.Errorf("could not push chunk locator to chunks queue: %w", err)
	}
	if !ok {
		log.Debug().Msg("could not push duplicate chunk locator to chunks queue")
		return nil
	}

	e.metrics.OnChunkProcessed()

	// notifies chunk queue consumer of a new chunk
	e.newChunkListener.Check()
	log.Debug().Msg("chunk locator successfully pushed to chunks queue")

	return nil
}

// ProcessFinalizedBlock is the entry point of assigner engine. It pushes the block down the pipeline with tracing on it enabled.
// Through the pipeline the execution receipts included in the block are indexed, and their chunk assignments are done, and
// the assigned chunks are pushed to the chunks queue, which is the output stream of this engine.
// Once the assigner engine is done handling all the receipts in the block, it notifies the block consumer.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	blockID := block.ID()
	span, ok := e.tracer.GetSpan(blockID, trace.VERProcessFinalizedBlock)
	if !ok {
		span = e.tracer.StartSpan(blockID, trace.VERProcessFinalizedBlock)
		span.SetTag("block_id", blockID)
		defer span.Finish()
	}

	ctx := opentracing.ContextWithSpan(e.unit.Ctx(), span)
	e.tracer.WithSpanFromContext(ctx, trace.VERAssignerHandleFinalizedBlock, func() {
		e.processFinalizedBlock(ctx, block)
	})
}

// processFinalizedBlock indexes the execution receipts included in the block, performs chunk assignment on its result, and
// processes the chunks assigned to this verification node by pushing them to the chunks consumer.
func (e *Engine) processFinalizedBlock(ctx context.Context, block *flow.Block) {
	blockID := block.ID()
	// we should always notify block consumer before returning.
	defer e.blockConsumerNotifier.Notify(blockID)

	log := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Uint64("block_height", block.Header.Height).
		Int("receipt_num", len(block.Payload.Receipts)).Logger()

	log.Debug().Msg("new finalized block arrived")

	err := e.indexer.Index(block.Payload.Receipts)
	if err != nil {
		// TODO: consider aborting the process
		log.Error().Err(err).Msg("could not index receipts for block")
	}

	// performs chunk assigment on each receipt and pushes the assigned chunks to the
	// chunks queue.
	for _, receipt := range block.Payload.Receipts {
		chunkList, err := e.receiptChunkAssignmentWithTracing(ctx, receipt, blockID)
		resultID := receipt.ExecutionResult.ID()
		if err != nil {
			log.Fatal().
				Err(err).
				Hex("receipt_id", logging.ID(receipt.ID())).
				Hex("result_id", logging.ID(resultID)).
				Hex("executor_id", logging.ID(receipt.ExecutorID)).
				Msg("could not determine assigned chunks of the receipt")
		}

		for _, chunk := range chunkList {
			err := e.processChunkWithTracing(ctx, chunk, resultID)
			if err != nil {
				log.Fatal().
					Err(err).
					Hex("result_id", logging.ID(resultID)).
					Hex("chunk_id", logging.ID(chunk.ID())).
					Uint64("chunk_index", chunk.Index).
					Msg("could not process chunk")
			}
		}
	}

	e.metrics.OnAssignerProcessFinalizedBlock(block.Header.Height)
	log.Info().Msg("finished processing finalized block")
}

// chunkAssignments returns the list of chunks in the chunk list assigned to this verification node.
func (e *Engine) chunkAssignments(ctx context.Context, result *flow.ExecutionResult) (flow.ChunkList, error) {
	var span opentracing.Span
	span, _ = e.tracer.StartSpanFromContext(ctx, trace.VERMatchMyChunkAssignments)
	defer span.Finish()

	assignment, err := e.assigner.Assign(result, result.BlockID)
	if err != nil {
		return nil, err
	}

	mine, err := assignedChunks(e.me.NodeID(), assignment, result.Chunks)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

// stakedAsVerification checks whether this instance of verification node has staked at specified block ID.
// It returns true and nil if verification node is staked at referenced block ID, and returns false and nil otherwise.
// It returns false and error if it could not extract the stake of node as a verification node at the specified block.
func stakedAsVerification(state protocol.State, blockID flow.Identifier, identifier flow.Identifier) (bool, error) {
	// TODO define specific error for handling cases
	identity, err := state.AtBlockID(blockID).Identity(identifier)
	if err != nil {
		return false, nil
	}

	// checks role of node is verification
	if identity.Role != flow.RoleVerification {
		return false, fmt.Errorf("node is staked for an invalid role. expected: %s, got: %s", flow.RoleVerification, identity.Role)
	}

	// checks identity has not been ejected
	if identity.Ejected {
		return false, nil
	}

	// checks identity has stake
	if identity.Stake == 0 {
		return false, nil
	}

	return true, nil
}

// receiptChunkAssignmentWithTracing handles the chunk assignment of a receipt in a container block with tracing enabled.
func (e *Engine) receiptChunkAssignmentWithTracing(ctx context.Context, receipt *flow.ExecutionReceipt,
	containerBlockID flow.Identifier) (flow.ChunkList, error) {
	var err error
	var chunkList flow.ChunkList
	e.tracer.WithSpanFromContext(ctx, trace.VERAssignerHandleExecutionReceipt, func() {
		chunkList, err = e.receiptChunkAssignment(ctx, receipt, containerBlockID)
	})
	return chunkList, err
}

// processChunkWithTracing receives a chunks belong to the same execution result and processes it with tracing enabled.
//
// Note that the chunk in the input should be legitimately assigned to this verification node
// (through the chunk assigner), and belong to the same execution result.
func (e *Engine) processChunkWithTracing(ctx context.Context, chunk *flow.Chunk, resultID flow.Identifier) error {
	var err error
	e.tracer.WithSpanFromContext(ctx, trace.VERAssignerProcessChunk, func() {
		err = e.processChunk(chunk, resultID)
	})
	return err
}

// assignedChunks returns the chunks assigned to a specific assignee based on the input chunk assignment.
func assignedChunks(assignee flow.Identifier, assignment *chunks.Assignment, chunks flow.ChunkList) (flow.ChunkList, error) {
	// indices of chunks assigned to verifier
	chunkIndices := assignment.ByNodeID(assignee)

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
