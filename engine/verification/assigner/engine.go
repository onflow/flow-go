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
	assigner              module.ChunkAssigner  // to determine chunks this node should verify.
	chunksQueue           storage.ChunksQueue   // to store chunks to be verified.
	newChunkListener      module.NewJobListener // to notify chunk queue consumer about a new chunk.
	blockConsumerNotifier ProcessingNotifier    // to report a block has been processed.
	indexer               storage.Indexer       // to index receipts of a block based on their executor identifier.
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
	indexer storage.Indexer,
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

func (e *Engine) withBlockConsumerNotifier(notifier ProcessingNotifier) {
	e.blockConsumerNotifier = notifier
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
	return fmt.Errorf("assigner engine is not supposed to invoked by process method")
}

// handleExecutionReceipt receives a receipt that appears in a finalized container block. In case this verification node
// is staked at the reference block of this execution receipt's result, chunk assignment is done on the execution result, and
// the assigned chunks' locators are pushed to the chunks queue, which are made available for the chunk queue consumer (i.e., the
// fetcher engine).
func (e *Engine) handleExecutionReceipt(receipt *flow.ExecutionReceipt, containerBlockID flow.Identifier) {
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
	ok, err := e.stakedAtBlockID(referenceBlockID)
	if err != nil {
		log.Error().Err(err).Msg("could not verify stake of verification node for result at reference block id")
		return
	}
	if !ok {
		log.Debug().Msg("node is not staked at reference block id, receipt is discarded")
		return
	}

	// chunk assignment
	chunkList, err := e.chunkAssignments(context.Background(), &receipt.ExecutionResult)
	if err != nil {
		log.Error().Err(err).Msg("could not determine chunk assignment")
		return
	}
	e.metrics.OnChunksAssigned(len(chunkList))
	log.Info().
		Int("total_assigned_chunks", len(chunkList)).
		Msg("chunk assignment done")

	// pushes assigned chunks to
	e.processChunks(chunkList, resultID)
}

// processChunks receives a list of chunks all belong to the same execution result. It creates a chunk locator
// for each of those chunks and stores the chunk locator in the chunks queue.
//
// Note that all chunks in the input chunk list are assume to be legitimately assigned to this verification node
// (through the chunk assigner), and all belong to the same execution result.
//
// Deduplication of chunk locators is delegated to the chunks queue.
func (e *Engine) processChunks(chunkList flow.ChunkList, resultID flow.Identifier) {
	for _, chunk := range chunkList {
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
			log.Error().Err(err).Msg("could not push chunk locator to chunks queue")
			continue
		}
		if !ok {
			log.Debug().Msg("could not push duplicate chunk locator to chunks queue")
			continue
		}

		e.metrics.OnChunkProcessed()

		// notifies chunk queue consumer of a new chunk
		e.newChunkListener.Check()
		log.Debug().Msg("chunk locator successfully pushed to chunks queue")
	}
}

// stakedAtBlockID checks whether this instance of verification node has staked at specified block ID.
// It returns true and nil if verification node is staked at referenced block ID, and returns false and nil otherwise.
// It returns false and error if it could not extract the stake of node as a verification node at the specified block.
func (e *Engine) stakedAtBlockID(blockID flow.Identifier) (bool, error) {
	identity, err := protocol.IdentityAtBlockID(e.state, blockID, e.me.NodeID())
	if err != nil {
		return false, fmt.Errorf("could not extract staked identify of node at block %v: %w", blockID, err)
	}

	if identity.Role != flow.RoleVerification {
		return false, fmt.Errorf("node is staked for an invalid role. expected: %s, got: %s", flow.RoleVerification, identity.Role)
	}

	staked := identity.Stake > 0
	return staked, nil
}

// ProcessFinalizedBlock indexes the execution receipts included in the block, and handles their chunk assignments.
// Once it is done handling all the receipts in the block, it notifies the block consumer.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	blockID := block.ID()
	log := e.log.With().
		Hex("block_id", logging.ID(blockID)).
		Int("receipt_num", len(block.Payload.Receipts)).Logger()

	log.Debug().Msg("new finalized block arrived")

	e.metrics.OnFinalizedBlockReceived()

	err := e.indexer.IndexReceipts(blockID)
	if err != nil {
		// TODO: consider aborting the process
		log.Error().Err(err).Msg("could not index receipts for block")
	}

	for _, receipt := range block.Payload.Receipts {
		e.handleExecutionReceipt(receipt, blockID)
	}

	// tells block consumer that it is done with this block
	e.blockConsumerNotifier.Notify(blockID)
	log.Debug().Msg("finished processing finalized block")
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
