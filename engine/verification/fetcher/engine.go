package fetcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Fetch engine processes each chunk in the chunk job queue, fetches its chunk data pack
// from the execution nodes who produced the receipts, and when the chunk data pack are
// received, it passes the verifiable chunk data to verifier engine to verify the chunk.
type Engine struct {
	unit                  *engine.Unit
	log                   zerolog.Logger
	metrics               module.VerificationMetrics
	tracer                module.Tracer
	me                    module.Local
	verifier              network.Engine            // the verifier engine
	state                 protocol.State            // used to verify the request origin
	pendingChunks         *Chunks                   // used to store all the pending chunks that assigned to this node
	headers               storage.Headers           // used to fetch the block header when chunk data is ready to be verified
	chunkConsumerNotifier module.ProcessingNotifier // to report a chunk has been processed
	results               storage.ExecutionResults  // to retrieve execution result of an assigned chunk
	receiptsDB            storage.ExecutionReceipts // used to find executor of the chunk
	requester             ChunkDataPackRequester    // used to request chunk data packs from network
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	me module.Local,
	verifier network.Engine,
	state protocol.State,
	chunks *Chunks,
	headers storage.Headers,
) (*Engine, error) {
	e := &Engine{
		unit:          engine.NewUnit(),
		metrics:       metrics,
		tracer:        tracer,
		log:           log.With().Str("engine", "fetcher").Logger(),
		me:            me,
		verifier:      verifier,
		state:         state,
		pendingChunks: chunks,
		headers:       headers,
	}

	return e, nil
}

func (e *Engine) WithChunkConsumerNotifier(notifier module.ProcessingNotifier) {
	e.chunkConsumerNotifier = notifier
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	if e.chunkConsumerNotifier == nil {
		panic("missing chunk consumer notifier callback in verification fetcher engine")
	}
}

// Done terminates the engine and returns a channel that is closed when the termination is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// ProcessAssignedChunk is the entry point of fetcher engine.
// It pushes the assigned chunk down the pipeline with tracing on it enabled.
// Through the pipeline the chunk data pack for this chunk is requested, a verifiable chunk is shaped for it,
// and is pushed to the verifier engine for verification.
func (e *Engine) ProcessAssignedChunk(locator *chunks.Locator) {
	span, ok := e.tracer.GetSpan(locator.ResultID, trace.VERProcessExecutionResult)
	if !ok {
		span = e.tracer.StartSpan(locator.ResultID, trace.VERProcessExecutionResult)
		span.SetTag("result_id", locator.ResultID)
		defer span.Finish()
	}

	ctx := opentracing.ContextWithSpan(e.unit.Ctx(), span)
	e.tracer.WithSpanFromContext(ctx, trace.VERFetcherHandleChunkLocator, func() {
		e.processAssignedChunkWithTracing(ctx, locator)
	})
}

//
func (e *Engine) processAssignedChunkWithTracing(ctx context.Context, locator *chunks.Locator) {
	log := e.log.With().
		Hex("chunk_locator_id", logging.ID(locator.ID())).
		Hex("result_id", logging.ID(locator.ResultID)).
		Uint64("chunk_index", locator.Index).
		Logger()

	log.Debug().Msg("new assigned chunk locator arrived")

	result, err := e.results.ByID(locator.ResultID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not retrieve result for chunk locator")
	}
	chunk := result.Chunks[locator.Index]

}

// ProcessMyChunk processes the chunk that assigned to me. It should not be blocking since
// multiple workers might be calling it concurrently.
// It skips chunks for sealed blocks.
// It fetches the chunk data pack, once received, verifier engine will be verifying
// Once a chunk has been processed, it will call the processing notifier callback to notify
// the chunk consumer in order to process the next chunk.
func (e *Engine) ProcessMyChunk(c *flow.Chunk, resultID flow.Identifier) {
	chunkID := c.ID()
	blockID := c.ChunkBody.BlockID
	lg := e.log.With().
		Hex("chunk", chunkID[:]).
		Hex("block", blockID[:]).
		Hex("result_id", resultID[:]).
		Logger()

	sealed, header, err := blockIsSealed(e.state, e.headers, blockID)

	if err != nil {
		lg.Error().Err(err).Msg("could not check if block is sealed")
		e.chunkConsumerNotifier.Notify(chunkID)
		return
	}

	// skip sealed blocks
	if sealed {
		lg.Debug().Msg("skip sealed chunk")
		e.chunkConsumerNotifier.Notify(chunkID)
		return
	}

	lg = lg.With().Uint64("height", header.Height).Logger()

	err = e.processChunk(c, header, resultID)

	if err != nil {
		lg.Error().Err(err).Msg("could not process chunk")
		// we report finish processing this chunk even if it failed
		e.chunkConsumerNotifier.Notify(chunkID)
	} else {
		lg.Info().Msgf("processing chunk")
	}
}

func (e *Engine) processChunk(index uint64, blockID flow.Identifier) {

}

func (e *Engine) processChunk(c *flow.Chunk, header *flow.Header, resultID flow.Identifier) error {
	blockID := c.ChunkBody.BlockID
	receiptsData, err := e.receiptsDB.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not retrieve receipts for block: %v: %w", blockID, err)
	}

	var receipts []*flow.ExecutionReceipt
	copy(receipts, receiptsData)

	agrees, disagrees := executorsOf(receipts, resultID)
	// chunk data pack request will only be sent to executors who produced the same result,
	// never to who produced different results.
	status := NewChunkStatus(c, resultID, header.Height, agrees, disagrees)
	added := e.pendingChunks.Add(status)
	if !added {
		return nil
	}

	allExecutors, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not find executors: %w", err)
	}

	err = e.requestChunkDataPack(status, allExecutors)
	if err != nil {
		return fmt.Errorf("could not request chunk data pack: %w", err)
	}

	// requesting a chunk data pack is async, when we receive it
	// we will resume processing, and eventually call Notify
	// again.
	// in case we never receive the chunk data pack response, we need
	// to make sure Notify is still called, because the
	// consumer is still waiting for it to report finish processing,
	return nil
}

func (e *Engine) onChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) error {

}

func (e *Engine) validateChunkDataPack(
	chunk *flow.Chunk,
	senderID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection,
) error {
	// 1. sender must be an execution node at that block
	blockID := chunk.BlockID
	sender, err := e.state.AtBlockID(blockID).Identity(senderID)
	if errors.Is(err, storage.ErrNotFound) {
		return engine.NewInvalidInputErrorf("sender is usntaked: %v", senderID)
	}

	if err != nil {
		return fmt.Errorf("could not find identity for chunk: %w", err)
	}

	if sender.Role != flow.RoleExecution {
		return engine.NewInvalidInputErrorf("sender is not execution node: %v", sender.Role)
	}

	// 2. start state must match
	if !bytes.Equal(chunkDataPack.StartState, chunk.ChunkBody.StartState) {
		return engine.NewInvalidInputErrorf(
			"expecting chunk data pakc's start state: %v, but got: %v",
			chunk.ChunkBody.StartState, chunkDataPack.StartState)
	}

	// 3. collection id must match
	collID := collection.ID()
	if chunkDataPack.CollectionID != collID {
		return engine.NewInvalidInputErrorf("mismatch collection id, %v != %v",
			chunkDataPack.CollectionID, collID)
	}

	return nil
}

func (e *Engine) getResultByID(blockID flow.Identifier, resultID flow.Identifier) (*flow.ExecutionResult, error) {
	receipts, err := e.receiptsDB.ByBlockID(blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get receipts by block ID: %w", err)
	}

	for _, receipt := range receipts {
		if receipt.ExecutionResult.ID() == resultID {
			return &receipt.ExecutionResult, nil
		}
	}

	return nil, fmt.Errorf("no receipt found for the result %v", blockID)
}

// HandleChunkDataPack is called by the chunk requester module everytime a new request chunk arrives.
// The chunks are supposed to be deduplicated by the requester. So invocation of this method indicates arrival of a distinct
// requested chunk.
func (e *Engine) HandleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) {
	e.handleChunkDataPack(originID, chunkDataPack, collection)
}

// HandleChunkDataPack is called by the chunk requester module everytime a new request chunk arrives.
// The chunks are supposed to be deduplicated by the requester.
// So invocation of this method indicates arrival of a distinct requested chunk.
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) {
	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
		Hex("collection_id", logging.ID(collection.ID())).
		Logger()

	log.Info().Msg("chunk data pack arrived")

	chunk, result, err := e.validatedAndFetch(originID, chunkDataPack, collection)
	if err != nil {
		log.Fatal().Err(err).Msg("could not validate and fetch chunk status")
	}
	log = log.With().
		Hex("result_id", logging.ID(result.ID())).
		Hex("block_id", logging.ID(chunk.BlockID)).Logger()

	removed := e.chunkStatusCleanUp(chunkDataPack.ChunkID)
	log.Debug().Bool("removed", removed).Msg("removed chunk status")

	// we need to report that the job has been finished eventually
	defer e.chunkConsumerNotifier.Notify(chunkDataPack.ChunkID)

	err = e.pushToVerifier(chunk, result, chunkDataPack, collection)
	if err != nil {
		log.Fatal().Err(err).Msg("could not push the chunk to verifier engine")
	}
	log.Info().Msg("verifiable chunk successfully pushed to verifier engine")
}

// NotifyChunkDataPackSealed is called by the ChunkDataPackRequester to notify the ChunkDataPackHandler that the chunk ID has been sealed and
// hence the requester will no longer request it.
//
// When the requester calls this callback method, it will never returns a chunk data pack for this chunk ID to the handler (i.e.,
// through HandleChunkDataPack).
func (e *Engine) NotifyChunkDataPackSealed(chunkID flow.Identifier) {
	// we need to report that the job has been finished eventually
	defer e.chunkConsumerNotifier.Notify(chunkID)
	removed := e.chunkStatusCleanUp(chunkID)

	e.log.Info().Bool("removed", removed).Msg("discards fetching chunk of an already sealed block")
}

// chunkStatusCleanUp is called whenever the engine is done processing a chunk. It cleans up the resources allocated for this chunk.
func (e *Engine) chunkStatusCleanUp(chunkID flow.Identifier) bool {
	return e.pendingChunks.Rem(chunkID)
}

// validatedAndFetch validates the chunk data pack and if it passes the validation, retrieves and returns its chunk as well as the
// execution result.
func (e *Engine) validatedAndFetch(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) (*flow.Chunk,
	*flow.ExecutionResult, error) {
	chunkID := chunkDataPack.ChunkID

	// make sure we still need it
	status, exists := e.pendingChunks.ByID(chunkID)
	if !exists {
		return nil, nil, fmt.Errorf("could not fetch chunk data from mempool: %x", chunkID)
	}

	// make sure the chunk data pack is valid
	err := e.validateChunkDataPack(status.Chunk, originID, chunkDataPack, collection)
	if err != nil {
		return nil, nil, engine.NewInvalidInputErrorf("invalid chunk data pack for chunk: %v collection: %v block: %v",
			chunkID, collection.ID(), status.Chunk.BlockID)
	}

	result, err := e.getResultByID(status.Chunk.BlockID, status.ExecutionResultID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get result by id %v: %w", result.ID(), err)
	}

	return status.Chunk, result, nil
}

// pushToVerifier makes a verifiable chunk data out of the input and pass it to the verifier for verification.
func (e *Engine) pushToVerifier(chunk *flow.Chunk, result *flow.ExecutionResult, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) error {
	header, err := e.headers.ByBlockID(chunk.BlockID)
	if err != nil {
		return fmt.Errorf("could not get block header: %w", err)
	}

	vchunk, err := e.makeVerifiableChunkData(chunk, header, result, chunkDataPack, collection)
	if err != nil {
		return fmt.Errorf("could not make verifiable chunk data: %w", err)
	}

	err = e.verifier.ProcessLocal(vchunk)
	if err != nil {
		return fmt.Errorf("verifier could not verify chunk: %w", err)
	}

	return nil
}

// makeVerifiableChunkData creates and returns a verifiable chunk data for the chunk data.
// The verifier engine, which is the last engine in the pipeline of verification, uses this verifiable
// chunk data to verify it.
func (e *Engine) makeVerifiableChunkData(chunk *flow.Chunk,
	header *flow.Header,
	result *flow.ExecutionResult,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection,
) (*verification.VerifiableChunkData, error) {

	// system chunk is the last chunk
	isSystemChunk := IsSystemChunk(chunk.Index, result)
	// computes the end state of the chunk
	var endState flow.StateCommitment
	if isSystemChunk {
		// last chunk in a result is the system chunk and takes final state commitment
		var ok bool
		endState, ok = result.FinalStateCommitment()
		if !ok {
			return nil, fmt.Errorf("fatal: can not read final state commitment, likely a bug")
		}
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.Chunks[chunk.Index+1].StartState
	}

	return &verification.VerifiableChunkData{
		IsSystemChunk: isSystemChunk,
		Chunk:         chunk,
		Header:        header,
		Result:        result,
		Collection:    collection,
		ChunkDataPack: chunkDataPack,
		EndState:      endState,
	}, nil
}

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}

func (e *Engine) requestChunkDataPack(chunkID flow.Identifier, resultID flow.Identifier, blockID flow.Identifier) error {

	request := &ChunkDataPackRequest{
		ChunkID:   flow.Identifier{},
		Height:    0,
		Agrees:    nil,
		Disagrees: nil,
	}
}

// getAgreeAndDisagreeExecutors segregates the execution nodes identifiers based on the given execution result id at the given block into agree and
// disagree sets.
// The agree set contains the executors who made receipt with the same result as the given result id.
// The disagree set contains the executors who made receipt with different result than the given result id.
func (e *Engine) getAgreeAndDisagreeExecutors(blockID flow.Identifier, resultID flow.Identifier) (flow.IdentifierList, flow.IdentifierList, error) {
	receiptsData, err := e.receiptsDB.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve receipts for block: %v: %w", blockID, err)
	}

	var receipts []*flow.ExecutionReceipt
	copy(receipts, receiptsData)

	agrees, disagrees := executorsOf(receipts, resultID)
	return agrees, disagrees, nil
}

// executorsOf segregates the executors of the given receipts based on the given execution result id.
// The agree set contains the executors who made receipt with the same result as the given result id.
// The disagree set contains the executors who made receipt with different result than the given result id.
func executorsOf(receipts []*flow.ExecutionReceipt, resultID flow.Identifier) (flow.IdentifierList, flow.IdentifierList) {
	var agrees flow.IdentifierList
	var disagrees flow.IdentifierList

	for _, receipt := range receipts {
		executor := receipt.ExecutorID
		if receipt.ExecutionResult.ID() == resultID {
			agrees = append(agrees, executor)
		} else {
			disagrees = append(disagrees, executor)
		}
	}

	return agrees, disagrees
}
