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
	unit             *engine.Unit
	log              zerolog.Logger
	metrics          module.VerificationMetrics
	tracer           module.Tracer
	me               module.Local
	state            protocol.State
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
	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
) *Engine {
	e := &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "assigner").Logger(),
		metrics:          metrics,
		tracer:           tracer,
		me:               me,
		state:            state,
		assigner:         assigner,
		chunksQueue:      chunksQueue,
		newChunkListener: newChunkListener,
	}

	return e
}

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

	//
	ok, err := e.preprocess(receipt)
	if err != nil {
		log.Debug().Err(err).Msg("could not determine processability of receipt")
		return
	}
	if !ok {
		log.Debug().Err(err).Msg("receipt is not preprocessable, skipping")
		return
	}

	// chunk assignment
	chunkList, err := e.chunkAssignments(context.Background(), &receipt.ExecutionResult)
	if err != nil {
		log.Debug().Err(err).Msg("could not determine chunk assignment")
		return
	}
	log.Info().
		Int("total_assigned_chunks", len(chunkList)).
		Msg("chunk assignment done")
	if chunkList.Empty() {
		// no chunk is assigned to this verification node
		return
	}

	// pushes assigned chunks to
	e.processChunks(chunkList, resultID)
}

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

		ok, err := e.chunksQueue.StoreChunkLocator(locator)

		if err != nil {
			log.Debug().Err(err).Msg("could not push chunk to chunks queue")
			continue
		}

		if !ok {
			log.Debug().Msg("could not push chunk to chunks queue")
			continue
		}

		e.newChunkListener.Check()
		log.Debug().Msg("chunk successfully pushed to chunks queue")
	}
}

// preprocess performs initial evaluations on the receipt. It returns true if all following conditions satisfied.
// 1- node is staked at the reference block of the execution result of the receipt.
func (e *Engine) preprocess(receipt *flow.ExecutionReceipt) (bool, error) {
	log := log.With().
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("result_id", logging.Entity(receipt.ExecutionResult)).
		Hex("reference_block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Logger()

	// verification node should be staked at the reference block id.
	ok, err := e.stakedAtBlockID(receipt.ExecutionResult.BlockID)
	if err != nil {
		return false, fmt.Errorf("could not verify stake of verification node for result: %w", err)
	}

	if !ok {
		log.Debug().Msg("node is not staked at reference block id")
		return false, nil
	}

	return true, nil
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

// stakedAtBlockID checks whether this instance of verification node has staked at specified block ID.
// It returns true an  nil if verification node has staked at specified block ID, and returns false, and nil otherwise.
// It returns false and error if it could not extract the stake of (verification node) node at the specified block.
func (e *Engine) stakedAtBlockID(blockID flow.Identifier) (bool, error) {
	// extracts identity of verification node at block height of result
	identity, err := protocol.StakedIdentity(e.state, blockID, e.me.NodeID())
	if err != nil {
		return false, fmt.Errorf("could not check if node is staked at block %v: %w", blockID, err)
	}

	if identity.Role != flow.RoleVerification {
		return false, fmt.Errorf("node is staked for an invalid role: %s", identity.Role)
	}

	staked := identity.Stake > 0
	return staked, nil
}

// ProcessFinalizedBlock process each finalized block, and find the chunks that
// assigned to me, and store it to the chunks job queue.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	blockID := block.ID()
	e.log.Debug().
		Hex("block_id", logging.ID(blockID)).
		Int("receipt_num", len(block.Payload.Receipts)).
		Msg("new finalized block arrived")
	for _, receipt := range block.Payload.Receipts {
		e.handleExecutionReceipt(receipt, blockID)
	}

	// tells block consumer that it is done with this block
	e.finishProcessing.FinishProcessing(blockID)
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
