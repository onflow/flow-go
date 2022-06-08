package fetcher

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Engine implements the fetcher engine functionality. It works between a chunk consumer queue, and a verifier engine.
// Its input is an assigned chunk locator from the chunk consumer it is subscribed to.
//
// Its output is a verifiable chunk that it passes to the verifier engine.
//
// Fetcher engine is an AssignedChunkProcessor implementation: it receives assigned chunks to this verification node from the chunk consumer.
// The assigned chunks are passed on concurrent executions of its ProcessAssignedChunk method.
//
// On receiving an assigned chunk, the engine requests their chunk data pack through the requester that is attached to it.
// On receiving a chunk data pack response, the fetcher engine validates it, and shapes a verifiable chunk out of it, and passes it
// to the verifier engine.
type Engine struct {
	// common
	unit  *engine.Unit
	state protocol.State // used to verify the origin ID of chunk data response, and sealing status.

	// monitoring
	log     zerolog.Logger
	tracer  module.Tracer
	metrics module.VerificationMetrics

	// memory and storage
	pendingChunks mempool.ChunkStatuses     // stores all pending chunks that their chunk data is requested from requester.
	blocks        storage.Blocks            // used to for verifying collection ID.
	headers       storage.Headers           // used for building verifiable chunk data.
	results       storage.ExecutionResults  // used to retrieve execution result of an assigned chunk.
	receipts      storage.ExecutionReceipts // used to find executor ids of a chunk, for requesting chunk data pack.

	// output interfaces
	verifier              network.Engine            // used to push verifiable chunk down the verification pipeline.
	requester             ChunkDataPackRequester    // used to request chunk data packs from network.
	chunkConsumerNotifier module.ProcessingNotifier // used to notify chunk consumer that it is done processing a chunk.
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	verifier network.Engine,
	state protocol.State,
	pendingChunks mempool.ChunkStatuses,
	headers storage.Headers,
	blocks storage.Blocks,
	results storage.ExecutionResults,
	receipts storage.ExecutionReceipts,
	requester ChunkDataPackRequester,
) *Engine {
	e := &Engine{
		unit:          engine.NewUnit(),
		metrics:       metrics,
		tracer:        tracer,
		log:           log.With().Str("engine", "fetcher").Logger(),
		verifier:      verifier,
		state:         state,
		pendingChunks: pendingChunks,
		blocks:        blocks,
		headers:       headers,
		results:       results,
		receipts:      receipts,
		requester:     requester,
	}

	e.requester.WithChunkDataPackHandler(e)

	return e
}

// WithChunkConsumerNotifier sets the processing notifier of fetcher.
// The fetcher engine uses this notifier to inform the chunk consumer that it is done processing a given chunk, and
// is ready to receive a new chunk to process.
func (e *Engine) WithChunkConsumerNotifier(notifier module.ProcessingNotifier) {
	e.chunkConsumerNotifier = notifier
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	if e.chunkConsumerNotifier == nil {
		e.log.Fatal().Msg("missing chunk consumer notifier callback in verification fetcher engine")
	}
	return e.unit.Ready(func() {
		<-e.requester.Ready()
	})
}

// Done terminates the engine and returns a channel that is closed when the termination is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.requester.Done()
	})
}

// ProcessAssignedChunk is the entry point of fetcher engine.
// It pushes the assigned chunk down the pipeline.
// Through the pipeline the chunk data pack for this chunk is requested,
// a verifiable chunk is shaped for it,
// and is pushed to the verifier engine for verification.
//
// It should not be blocking since multiple chunk consumer workers might be calling it concurrently.
// It fetches the chunk data pack, once received, verifier engine will be verifying
// Once a chunk has been processed, it will call the processing notifier callback to notify
// the chunk consumer in order to process the next chunk.
func (e *Engine) ProcessAssignedChunk(locator *chunks.Locator) {
	locatorID := locator.ID()
	lg := e.log.With().
		Hex("locator_id", logging.ID(locatorID)).
		Hex("result_id", logging.ID(locator.ResultID)).
		Uint64("chunk_index", locator.Index).
		Logger()

	e.metrics.OnAssignedChunkReceivedAtFetcher()

	// retrieves result and chunk using the locator
	result, err := e.results.ByID(locator.ResultID)
	if err != nil {
		// a missing result for a chunk locator is a fatal error potentially a database leak.
		lg.Fatal().Err(err).Msg("could not retrieve result for chunk locator")
	}
	chunk := result.Chunks[locator.Index]
	chunkID := chunk.ID()

	lg = lg.With().
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("block_id", logging.ID(chunk.ChunkBody.BlockID)).
		Logger()
	lg.Debug().Msg("result and chunk for locator retrieved")

	requested, blockHeight, err := e.processAssignedChunkWithTracing(chunk, result, locatorID)
	lg = lg.With().Uint64("block_height", blockHeight).Logger()

	if err != nil {
		lg.Fatal().Err(err).Msg("could not process assigned chunk")
	}

	lg.Info().Bool("requested", requested).Msg("assigned chunk processed successfully")

	if requested {
		e.metrics.OnChunkDataPackRequestSentByFetcher()
	}

}

// processAssignedChunkWithTracing encapsulates the logic of processing assigned chunk with tracing enabled.
func (e *Engine) processAssignedChunkWithTracing(chunk *flow.Chunk, result *flow.ExecutionResult, chunkLocatorID flow.Identifier) (bool, uint64, error) {

	span, _, isSampled := e.tracer.StartBlockSpan(e.unit.Ctx(), result.BlockID, trace.VERProcessAssignedChunk)
	if isSampled {
		span.SetTag("collection_index", chunk.CollectionIndex)
	}
	defer span.Finish()

	requested, blockHeight, err := e.processAssignedChunk(chunk, result, chunkLocatorID)

	return requested, blockHeight, err
}

// processAssignedChunk receives an assigned chunk and its result and requests its chunk data pack from requester.
// Boolean return value determines whether chunk data pack was requested or not.
func (e *Engine) processAssignedChunk(chunk *flow.Chunk, result *flow.ExecutionResult, chunkLocatorID flow.Identifier) (bool, uint64, error) {
	// skips processing a chunk if it belongs to a sealed block.
	chunkID := chunk.ID()
	sealed, blockHeight, err := e.blockIsSealed(chunk.ChunkBody.BlockID)
	if err != nil {
		return false, 0, fmt.Errorf("could not determine whether block has been sealed: %w", err)
	}
	if sealed {
		e.chunkConsumerNotifier.Notify(chunkLocatorID) // tells consumer that we are done with this chunk.
		return false, blockHeight, nil
	}

	// adds chunk status as a pending chunk to mempool.
	status := &verification.ChunkStatus{
		ChunkIndex:      chunk.Index,
		ExecutionResult: result,
		BlockHeight:     blockHeight,
	}
	added := e.pendingChunks.Add(status)
	if !added {
		return false, blockHeight, nil
	}

	err = e.requestChunkDataPack(chunk.Index, chunkID, result.ID(), chunk.BlockID)
	if err != nil {
		return false, blockHeight, fmt.Errorf("could not request chunk data pack: %w", err)
	}

	// requesting a chunk data pack is async, i.e., once engine reaches this point
	// it gracefully waits (unblocking) for the requested
	// till it either delivers us the requested chunk data pack
	// or cancels our request (when chunk belongs to a sealed block).
	//
	// both these events happen through requester module calling fetchers callbacks.
	// it is during those callbacks that we notify the consumer that we are done with this job.
	return true, blockHeight, nil
}

// HandleChunkDataPack is called by the chunk requester module everytime a new requested chunk data pack arrives.
// The chunks are supposed to be deduplicated by the requester.
// So invocation of this method indicates arrival of a distinct requested chunk.
func (e *Engine) HandleChunkDataPack(originID flow.Identifier, response *verification.ChunkDataPackResponse) {
	lg := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(response.Cdp.ChunkID)).
		Logger()

	if response.Cdp.Collection != nil {
		// non-system chunk data packs have non-nil collection
		lg = lg.With().
			Hex("collection_id", logging.ID(response.Cdp.Collection.ID())).
			Logger()
		lg.Info().Msg("chunk data pack arrived")
	} else {
		lg.Info().Msg("system chunk data pack arrived")
	}

	e.metrics.OnChunkDataPackArrivedAtFetcher()

	// make sure we still need it
	status, exists := e.pendingChunks.Get(response.Index, response.ResultID)
	if !exists {
		lg.Debug().Msg("could not fetch pending status from mempool, dropping chunk data")
		return
	}

	resultID := status.ExecutionResult.ID()
	lg = lg.With().
		Hex("block_id", logging.ID(status.ExecutionResult.BlockID)).
		Uint64("block_height", status.BlockHeight).
		Hex("result_id", logging.ID(resultID)).
		Uint64("chunk_index", status.ChunkIndex).
		Bool("system_chunk", IsSystemChunk(status.ChunkIndex, status.ExecutionResult)).
		Logger()

	span, ctx, _ := e.tracer.StartBlockSpan(context.Background(), status.ExecutionResult.BlockID, trace.VERFetcherHandleChunkDataPack)
	defer span.Finish()

	processed, err := e.handleChunkDataPackWithTracing(ctx, originID, status, response.Cdp)
	if IsChunkDataPackValidationError(err) {
		lg.Error().Err(err).Msg("could not validate chunk data pack")
		return
	}

	if err != nil {
		// TODO: byzantine fault
		lg.Fatal().Err(err).Msg("could not handle chunk data pack")
		return
	}

	if processed {
		e.metrics.OnVerifiableChunkSentToVerifier()

		// we need to report that the job has been finished eventually
		e.chunkConsumerNotifier.Notify(status.ChunkLocatorID())
		lg.Info().Msg("verifiable chunk pushed to verifier engine")
	}

}

// handleChunkDataPackWithTracing encapsulates the logic of handling chunk data pack with tracing enabled.
//
// Boolean returned value determines whether the chunk data pack passed validation and its verifiable chunk
// submitted to verifier.
// The first returned value determines non-critical errors (i.e., expected ones).
// The last returned value determines the critical errors that are unexpected, and should lead program to halt.
func (e *Engine) handleChunkDataPackWithTracing(
	ctx context.Context,
	originID flow.Identifier,
	status *verification.ChunkStatus,
	chunkDataPack *flow.ChunkDataPack) (bool, error) {

	// make sure the chunk data pack is valid
	err := e.validateChunkDataPackWithTracing(ctx, status.ChunkIndex, originID, chunkDataPack, status.ExecutionResult)
	if err != nil {
		return false, NewChunkDataPackValidationError(originID,
			status.ExecutionResult.ID(),
			status.ChunkIndex,
			chunkDataPack.ID(),
			chunkDataPack.ChunkID,
			chunkDataPack.Collection.ID(),
			err)
	}

	processed, err := e.handleValidatedChunkDataPack(ctx, status, chunkDataPack)
	if err != nil {
		return processed, fmt.Errorf("could not handle validated chunk data pack: %w", err)
	}

	return processed, nil
}

// handleValidatedChunkDataPack receives a validated chunk data pack, removes its status from the memory, and pushes a verifiable chunk for it to
// verifier engine.
// Boolean return value determines whether verifiable chunk pushed to verifier or not.
func (e *Engine) handleValidatedChunkDataPack(ctx context.Context,
	status *verification.ChunkStatus,
	chunkDataPack *flow.ChunkDataPack) (bool, error) {

	removed := e.pendingChunks.Rem(status.ChunkIndex, status.ExecutionResult.ID())
	if !removed {
		// we deduplicate the chunk data responses at this point, reaching here means a
		// duplicate chunk data response is under process concurrently, so we give up
		// on processing current one.
		return false, nil
	}

	// pushes chunk data pack to verifier, and waits for it to be verified.
	chunk := status.ExecutionResult.Chunks[status.ChunkIndex]
	err := e.pushToVerifierWithTracing(ctx, chunk, status.ExecutionResult, chunkDataPack)
	if err != nil {
		return false, fmt.Errorf("could not push the chunk to verifier engine")
	}

	return true, nil
}

// validateChunkDataPackWithTracing encapsulates the logic of validating a chunk data pack with tracing enabled.
func (e *Engine) validateChunkDataPackWithTracing(ctx context.Context,
	chunkIndex uint64,
	senderID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult) error {

	var err error
	e.tracer.WithSpanFromContext(ctx, trace.VERFetcherValidateChunkDataPack, func() {
		err = e.validateChunkDataPack(chunkIndex, senderID, chunkDataPack, result)
	})

	return err
}

// validateChunkDataPack validates the integrity of a received chunk data pack as well as the authenticity of its sender.
// Regarding the integrity: the chunk data pack should have a matching start state with the chunk itself, as well as a matching collection ID with the
// given collection.
//
// Regarding the authenticity: the chunk data pack should be coming from a sender that is an authorized execution node at the block of the chunk.
func (e *Engine) validateChunkDataPack(chunkIndex uint64,
	senderID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult) error {

	chunk := result.Chunks[chunkIndex]
	// 1. chunk ID of chunk data pack should map the chunk ID on execution result
	expectedChunkID := chunk.ID()
	if chunkDataPack.ChunkID != expectedChunkID {
		return fmt.Errorf("chunk ID of chunk data pack does not match corresponding chunk on execution result, expected: %x, got:%x",
			expectedChunkID, chunkDataPack.ChunkID)
	}

	// 2. sender must be a authorized execution node at that block
	blockID := chunk.BlockID
	authorized := e.validateAuthorizedExecutionNodeAtBlockID(senderID, blockID)
	if !authorized {
		return fmt.Errorf("unauthorized execution node sender at block ID: %x, resultID: %x, chunk ID: %x",
			blockID,
			result.ID(),
			chunk.ID())
	}

	// 3. start state must match
	if chunkDataPack.StartState != chunk.ChunkBody.StartState {
		return engine.NewInvalidInputErrorf("expecting chunk data pack's start state: %x, but got: %x, block ID: %x, resultID: %x, chunk ID: %x",
			chunk.ChunkBody.StartState,
			chunkDataPack.StartState,
			blockID,
			result.ID(),
			chunk.ID())
	}

	// 3. collection id must match
	err := e.validateCollectionID(chunkDataPack, result, chunk)
	if err != nil {
		return fmt.Errorf("could not validate collection: %w", err)
	}

	return nil
}

// validateCollectionID returns error for an invalid collection of a chunk data pack,
// and returns nil otherwise.
func (e Engine) validateCollectionID(
	chunkDataPack *flow.ChunkDataPack,
	result *flow.ExecutionResult,
	chunk *flow.Chunk) error {

	if IsSystemChunk(chunk.Index, result) {
		return e.validateSystemChunkCollection(chunkDataPack)
	}

	return e.validateNonSystemChunkCollection(chunkDataPack, chunk)
}

// validateSystemChunkCollection returns nil if the system chunk data pack has a nil collection.
func (e Engine) validateSystemChunkCollection(chunkDataPack *flow.ChunkDataPack) error {
	// collection of a system chunk should be nil
	if chunkDataPack.Collection != nil {
		return engine.NewInvalidInputErrorf("non-nil collection for system chunk, collection ID: %v, len: %d",
			chunkDataPack.Collection.ID(), chunkDataPack.Collection.Len())
	}

	return nil
}

// validateNonSystemChunkCollection returns nil if the collection is matching the non-system chunk data pack.
// A collection is valid against a non-system chunk if it has a matching ID with the
// collection ID of corresponding guarantee of the chunk in the referenced block payload.
func (e Engine) validateNonSystemChunkCollection(chunkDataPack *flow.ChunkDataPack, chunk *flow.Chunk) error {
	collID := chunkDataPack.Collection.ID()

	block, err := e.blocks.ByID(chunk.BlockID)
	if err != nil {
		return fmt.Errorf("could not get block: %w", err)
	}

	if block.Payload.Guarantees[chunk.Index].CollectionID != collID {
		return engine.NewInvalidInputErrorf("mismatch collection id with guarantee, expected: %v, got: %v",
			block.Payload.Guarantees[chunk.Index].CollectionID,
			collID)
	}

	return nil
}

// validateAuthorizedExecutionNodeAtBlockID validates sender ID of a chunk data pack response as an authorized
// execution node at the given block ID.
func (e Engine) validateAuthorizedExecutionNodeAtBlockID(senderID flow.Identifier, blockID flow.Identifier) bool {
	snapshot := e.state.AtBlockID(blockID)
	valid, err := protocol.IsNodeAuthorizedWithRoleAt(snapshot, senderID, flow.RoleExecution)

	if err != nil {
		e.log.Fatal().
			Err(err).
			Hex("block_id", logging.ID(blockID)).
			Hex("sender_id", logging.ID(senderID)).
			Msg("could not validate sender identity at specified block ID snapshot as execution node")
	}

	return valid
}

// NotifyChunkDataPackSealed is called by the ChunkDataPackRequester to notify the ChunkDataPackHandler that the specified chunk
// has been sealed and hence the requester will no longer request it.
//
// When the requester calls this callback method, it will never return a chunk data pack for this specified chunk to the handler (i.e.,
// through HandleChunkDataPack).
func (e *Engine) NotifyChunkDataPackSealed(chunkIndex uint64, resultID flow.Identifier) {
	lg := e.log.With().
		Uint64("chunk_index", chunkIndex).
		Hex("result_id", logging.ID(resultID)).
		Logger()

	// we need to report that the job has been finished eventually
	status, exists := e.pendingChunks.Get(chunkIndex, resultID)
	if !exists {
		lg.Debug().
			Msg("could not fetch pending status for sealed chunk from mempool, dropping chunk data")
		return
	}

	chunkLocatorID := status.ChunkLocatorID()
	lg = lg.With().
		Uint64("block_height", status.BlockHeight).
		Hex("result_id", logging.ID(status.ExecutionResult.ID())).Logger()
	removed := e.pendingChunks.Rem(chunkIndex, resultID)

	e.chunkConsumerNotifier.Notify(chunkLocatorID)
	lg.Info().
		Bool("removed", removed).
		Msg("discards fetching chunk of an already sealed block and notified consumer")
}

// pushToVerifierWithTracing encapsulates the logic of pushing a verifiable chunk to verifier engine with tracing enabled.
func (e *Engine) pushToVerifierWithTracing(
	ctx context.Context,
	chunk *flow.Chunk,
	result *flow.ExecutionResult,
	chunkDataPack *flow.ChunkDataPack) error {

	var err error
	e.tracer.WithSpanFromContext(ctx, trace.VERFetcherPushToVerifier, func() {
		err = e.pushToVerifier(chunk, result, chunkDataPack)
	})

	return err
}

// pushToVerifier makes a verifiable chunk data out of the input and pass it to the verifier for verification.
//
// When this method returns without any error, it means that the verification of the chunk at the verifier engine is done (either successfully,
// or unsuccessfully)
func (e *Engine) pushToVerifier(chunk *flow.Chunk,
	result *flow.ExecutionResult,
	chunkDataPack *flow.ChunkDataPack) error {

	header, err := e.headers.ByBlockID(chunk.BlockID)
	if err != nil {
		return fmt.Errorf("could not get block: %w", err)
	}

	vchunk, err := e.makeVerifiableChunkData(chunk, header, result, chunkDataPack)
	if err != nil {
		return fmt.Errorf("could not verify chunk: %w", err)
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
) (*verification.VerifiableChunkData, error) {

	// system chunk is the last chunk
	isSystemChunk := IsSystemChunk(chunk.Index, result)

	endState, err := EndStateCommitment(result, chunk.Index, isSystemChunk)
	if err != nil {
		return nil, fmt.Errorf("could not compute end state of chunk: %w", err)
	}

	transactionOffset, err := TransactionOffsetForChunk(result.Chunks, chunk.Index)
	if err != nil {
		return nil, fmt.Errorf("cannot compute transaction offset for chunk: %w", err)
	}

	return &verification.VerifiableChunkData{
		IsSystemChunk:     isSystemChunk,
		Chunk:             chunk,
		Header:            header,
		Result:            result,
		ChunkDataPack:     chunkDataPack,
		EndState:          endState,
		TransactionOffset: transactionOffset,
	}, nil
}

// requestChunkDataPack creates and dispatches a chunk data pack request to the requester engine.
func (e *Engine) requestChunkDataPack(chunkIndex uint64, chunkID flow.Identifier, resultID flow.Identifier, blockID flow.Identifier) error {
	agrees, disagrees, err := e.getAgreeAndDisagreeExecutors(blockID, resultID)
	if err != nil {
		return fmt.Errorf("could not segregate the agree and disagree executors for result: %x of block: %x", resultID, blockID)
	}

	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get header for block: %x", blockID)
	}

	allExecutors, err := e.state.AtBlockID(blockID).Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not fetch execution node ids at block %x: %w", blockID, err)
	}

	request := &verification.ChunkDataPackRequest{
		Locator: chunks.Locator{
			ResultID: resultID,
			Index:    chunkIndex,
		},
		ChunkDataPackRequestInfo: verification.ChunkDataPackRequestInfo{
			ChunkID:   chunkID,
			Height:    header.Height,
			Agrees:    agrees,
			Disagrees: disagrees,
			Targets:   allExecutors,
		},
	}

	e.requester.Request(request)

	return nil
}

// getAgreeAndDisagreeExecutors segregates the execution nodes identifiers based on the given execution result id at the given block into agree and
// disagree sets.
// The agree set contains the executors who made receipt with the same result as the given result id.
// The disagree set contains the executors who made receipt with different result than the given result id.
func (e *Engine) getAgreeAndDisagreeExecutors(blockID flow.Identifier, resultID flow.Identifier) (flow.IdentifierList, flow.IdentifierList, error) {
	receipts, err := e.receipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve receipts for block: %v: %w", blockID, err)
	}

	agrees, disagrees := executorsOf(receipts, resultID)
	return agrees, disagrees, nil
}

// blockIsSealed returns true if the block at specified height by block ID is sealed.
func (e Engine) blockIsSealed(blockID flow.Identifier) (bool, uint64, error) {
	// TODO: as an optimization, we can keep record of last sealed height on a local variable.
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return false, 0, fmt.Errorf("could not get block: %w", err)
	}

	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		return false, 0, fmt.Errorf("could not get last sealed: %w", err)
	}

	sealed := header.Height <= lastSealed.Height
	return sealed, header.Height, nil
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

// EndStateCommitment computes the end state of the given chunk.
func EndStateCommitment(result *flow.ExecutionResult, chunkIndex uint64, systemChunk bool) (flow.StateCommitment, error) {
	var endState flow.StateCommitment
	if systemChunk {
		var err error
		// last chunk in a result is the system chunk and takes final state commitment
		endState, err = result.FinalStateCommitment()
		if err != nil {
			return flow.DummyStateCommitment, fmt.Errorf("can not read final state commitment, likely a bug:%w", err)
		}
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.Chunks[chunkIndex+1].StartState
	}

	return endState, nil
}

// TransactionOffsetForChunk calculates transaction offset for a given chunk which is the index of the first
// transaction of this chunk within the whole block
func TransactionOffsetForChunk(chunks flow.ChunkList, chunkIndex uint64) (uint32, error) {
	if int(chunkIndex) > len(chunks)-1 {
		return 0, fmt.Errorf("chunk list out of bounds, len %d asked for chunk %d", len(chunks), chunkIndex)
	}
	var offset uint32 = 0
	for i := 0; i < int(chunkIndex); i++ {
		offset += uint32(chunks[i].NumberOfTransactions)
	}
	return offset, nil
}

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}
