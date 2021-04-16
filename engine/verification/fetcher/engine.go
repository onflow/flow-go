package fetcher

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
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
	unit                  *engine.Unit
	log                   zerolog.Logger
	metrics               module.VerificationMetrics
	tracer                module.Tracer
	verifier              network.Engine            // used to push verifiable chunk down the verification pipeline.
	state                 protocol.State            // used to verify the origin ID of chunk data response, and sealing status.
	pendingChunks         mempool.ChunkStatuses     // stores all pending chunks that their chunk data is requested from requester.
	headers               storage.Headers           // used to fetch the block header for building verifiable chunk data.
	chunkConsumerNotifier module.ProcessingNotifier // used to notify chunk consumer that it is done processing a chunk.
	results               storage.ExecutionResults  // used to retrieve execution result of an assigned chunk.
	receipts              storage.ExecutionReceipts // used to find executor ids of a chunk, for requesting chunk data pack.
	requester             ChunkDataPackRequester    // used to request chunk data packs from network.
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	verifier network.Engine,
	state protocol.State,
	pendingChunks mempool.ChunkStatuses,
	headers storage.Headers,
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
		headers:       headers,
		results:       results,
		receipts:      receipts,
		requester:     requester,
	}

	return e
}

func (e *Engine) WithChunkConsumerNotifier(notifier module.ProcessingNotifier) {
	e.chunkConsumerNotifier = notifier
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	if e.chunkConsumerNotifier == nil {
		e.log.Fatal().Msg("missing chunk consumer notifier callback in verification fetcher engine")
	}
	return e.unit.Ready()
}

// Done terminates the engine and returns a channel that is closed when the termination is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// ProcessAssignedChunk is the entry point of fetcher engine.
// It pushes the assigned chunk down the pipeline with tracing on it enabled.
// Through the pipeline the chunk data pack for this chunk is requested,
// a verifiable chunk is shaped for it,
// and is pushed to the verifier engine for verification.
//
// ProcessAssignedChunk processes the chunk that assigned to this verification node.
// It should not be blocking since multiple workers might be calling it concurrently.
// It fetches the chunk data pack, once received, verifier engine will be verifying
// Once a chunk has been processed, it will call the processing notifier callback to notify
// the chunk consumer in order to process the next chunk.
func (e *Engine) ProcessAssignedChunk(locator *chunks.Locator) {
	log := e.log.With().
		Uint64("chunk_index", locator.Index).
		Hex("result_id", logging.ID(locator.ResultID)).
		Logger()

	log.Info().Msg("chunk locator arrived")

	// retrieving result and chunk
	result, err := e.results.ByID(locator.ResultID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not retrieve result for chunk locator")
	}
	chunk := result.Chunks[locator.Index]
	chunkID := chunk.ID()

	log = log.With().
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("block_id", logging.ID(chunk.ChunkBody.BlockID)).
		Logger()

	log.Debug().Msg("result and chunk for locator retrieved")

	// if block has been sealed, then we can finish
	sealed, err := e.blockIsSealed(chunk.ChunkBody.BlockID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not determine whether block has been sealed")
	}

	if sealed {
		e.chunkConsumerNotifier.Notify(chunkID) // tells consumer that we are done with this chunk.
		log.Info().
			Msg("drops requesting chunk of a sealed block")
		return
	}

	// adds chunk status as a pending chunk to mempool.
	status := &verification.ChunkStatus{
		Chunk:             chunk,
		ExecutionResultID: locator.ResultID,
	}
	added := e.pendingChunks.Add(status)
	if !added {
		// unless chunk consumer fails (on a bug) to deduplicate the chunks, it should not pass
		// the same chunk locator twice to this fetcher engine.
		log.Warn().Msg("skips processing an already existing pending chunk, possible data race")
		e.chunkConsumerNotifier.Notify(chunkID) // tells consumer that we are done with this chunk.
		return
	}

	err = e.requestChunkDataPack(chunk.ID(), locator.ResultID, chunk.BlockID)
	if err != nil {
		log.Fatal().Err(err).Msg("could not request chunk data pack")
	}

	// requesting a chunk data pack is async, i.e., once engine reaches this point
	// it gracefully waits (unblocking) till it either delivers us the requested chunk data pack
	// or cancels our request (when chunk belongs to a sealed block).
	//
	// both these events happen through requester module calling fetchers callbacks.
	// it is during those callbacks that we notify the consumer that we are done with this job.
	log.Info().Msg("chunk data pack requested from requester engine")
}

func (e *Engine) validateChunkDataPack(
	chunk *flow.Chunk,
	senderID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection,
) error {
	// 1. sender must be an execution node at that block
	blockID := chunk.BlockID
	staked, err := e.validateStakedExecutionNodeAtBlockID(senderID, blockID)
	if err != nil {
		return fmt.Errorf("could not validate identity of sender at block ID as an execution node: %w", err)
	}

	if !staked {
		return fmt.Errorf("unstaked execution node sender at block ID")
	}

	// 2. start state must match
	if !bytes.Equal(chunkDataPack.StartState, chunk.ChunkBody.StartState) {
		return engine.NewInvalidInputErrorf(
			"expecting chunk data pack's start state: %v, but got: %v",
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

// validateStakedExecutionNodeAtBlockID validates sender ID of a chunk data pack response as an staked
// execution node  at the given block ID.
func (e Engine) validateStakedExecutionNodeAtBlockID(senderID flow.Identifier, blockID flow.Identifier) (bool, error) {

	sender, err := e.state.AtBlockID(blockID).Identity(senderID)
	if errors.Is(err, storage.ErrNotFound) {
		return false, engine.NewInvalidInputErrorf("sender is unstaked: %v", senderID)
	}
	if err != nil {
		return false, fmt.Errorf("could not find identity for chunk: %w", err)
	}

	if sender.Role != flow.RoleExecution {
		return false, engine.NewInvalidInputErrorf("sender is not execution node: %v", sender.Role)
	}

	return sender.Stake > 0, nil
}

// HandleChunkDataPack is called by the chunk requester module everytime a new request chunk arrives.
// The chunks are supposed to be deduplicated by the requester.
// So invocation of this method indicates arrival of a distinct requested chunk.
func (e *Engine) HandleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) {
	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
		Hex("collection_id", logging.ID(collection.ID())).
		Logger()

	log.Info().Msg("chunk data pack arrived")

	status, err := e.validatedStatus(originID, chunkDataPack, collection)
	if err != nil {
		// TODO: this can be due to a byzantine behavior
		log.Info().Err(err).Msg("could not validate and fetch chunk status")
		return
	}

	result, err := e.results.ByID(status.ExecutionResultID)
	if err != nil {
		// this error indicates a fatal situation that we are missing an execution result.
		// can be a database leakage.
		log.Fatal().Err(err).Msg("could not retrieve execution result of chunk status, possibly a bug")
		return
	}

	log = log.With().
		Hex("result_id", logging.ID(result.ID())).
		Hex("block_id", logging.ID(status.Chunk.BlockID)).Logger()

	removed := e.pendingChunks.Rem(chunkDataPack.ChunkID)
	if !removed {
		// not being able to remove it means a duplicate response is being processed concurrently
		// so we terminate processing the current one.
		log.Debug().Bool("removed", removed).Msg("removed chunk status")
		return
	}

	err = e.pushToVerifier(status.Chunk, result, chunkDataPack, collection)
	if err != nil {
		log.Fatal().Err(err).Msg("could not push the chunk to verifier engine")
	}
	// we need to report that the job has been finished eventually
	e.chunkConsumerNotifier.Notify(chunkDataPack.ChunkID)
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
	removed := e.pendingChunks.Rem(chunkID)

	e.log.Info().Bool("removed", removed).Msg("discards fetching chunk of an already sealed block")
}

// validatedStatus validates the chunk data pack and if it passes the validation, retrieves and returns its chunk status.
func (e *Engine) validatedStatus(originID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection) (*verification.ChunkStatus, error) {

	// make sure we still need it
	status, exists := e.pendingChunks.ByID(chunkDataPack.ChunkID)
	if !exists {
		return nil, fmt.Errorf("could not fetch chunk data from mempool: %x", chunkDataPack.ChunkID)
	}

	// make sure the chunk data pack is valid
	err := e.validateChunkDataPack(status.Chunk, originID, chunkDataPack, collection)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("invalid chunk data pack for chunk: %v collection: %v block: %v error:%w",
			chunkDataPack.ChunkID, collection.ID(), status.Chunk.BlockID, err)
	}

	return status, nil
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

	endState, err := EndStateCommitment(result, chunk.Index, isSystemChunk)
	if err != nil {
		return nil, fmt.Errorf("could not compute end state of chunk: %w", err)
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

// EndStateCommitment computes the end state of the given chunk.
func EndStateCommitment(result *flow.ExecutionResult, chunkIndex uint64, systemChunk bool) (flow.StateCommitment, error) {

	var endState flow.StateCommitment
	if systemChunk {
		// last chunk in a result is the system chunk and takes final state commitment
		var ok bool
		endState, ok = result.FinalStateCommitment()
		if !ok {
			return nil, fmt.Errorf("can not read final state commitment, likely a bug")
		}
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.Chunks[chunkIndex+1].StartState
	}

	return endState, nil
}

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}

// requestChunkDataPack creates and dispatches a chunk data pack request to the requester module of the engine.
func (e *Engine) requestChunkDataPack(chunkID flow.Identifier, resultID flow.Identifier, blockID flow.Identifier) error {
	agrees, disagrees, err := e.getAgreeAndDisagreeExecutors(blockID, resultID)
	if err != nil {
		return fmt.Errorf("could not segregate the agree and disagree executors for result: %x of block: %x", resultID, blockID)
	}

	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get header for block: %x", blockID)
	}

	request := &verification.ChunkDataPackRequest{
		ChunkID:   chunkID,
		Height:    header.Height,
		Agrees:    agrees,
		Disagrees: disagrees,
	}

	allExecutors, err := e.state.AtBlockID(blockID).Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not fetch execution node ids at block: %x", blockID)
	}

	e.requester.Request(request, allExecutors)
	return nil
}

// getAgreeAndDisagreeExecutors segregates the execution nodes identifiers based on the given execution result id at the given block into agree and
// disagree sets.
// The agree set contains the executors who made receipt with the same result as the given result id.
// The disagree set contains the executors who made receipt with different result than the given result id.
func (e *Engine) getAgreeAndDisagreeExecutors(blockID flow.Identifier, resultID flow.Identifier) (flow.IdentifierList, flow.IdentifierList, error) {
	receiptsData, err := e.receipts.ByBlockID(blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not retrieve receipts for block: %v: %w", blockID, err)
	}

	receipts := make([]*flow.ExecutionReceipt, len(receiptsData))
	copy(receipts, receiptsData)

	agrees, disagrees := executorsOf(receipts, resultID)
	return agrees, disagrees, nil
}

// blockIsSealed returns true if the block at specified height by block ID is sealed.
func (e Engine) blockIsSealed(blockID flow.Identifier) (bool, error) {
	// TODO: as an optimization, we can keep record of last sealed height on a local variable.
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return false, fmt.Errorf("could not get block header: %w", err)
	}

	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		return false, fmt.Errorf("could not get last sealed: %w", err)
	}

	sealed := header.Height <= lastSealed.Height
	return sealed, nil
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
