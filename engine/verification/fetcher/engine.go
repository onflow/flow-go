package fetcher

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// Fetch engine processes each chunk in the chunk job queue, fetches its chunk data pack
// from the execution nodes who produced the receipts, and when the chunk data pack are
// received, it passes the verifiable chunk data to verifier engine to verify the chunk.
type Engine struct {
	unit             *engine.Unit
	log              zerolog.Logger
	metrics          module.VerificationMetrics
	tracer           module.Tracer
	me               module.Local
	verifier         network.Engine   // the verifier engine
	state            protocol.State   // used to verify the request origin
	pendingChunks    *Chunks          // used to store all the pending chunks that assigned to this node
	con              network.Conduit  // used to send the chunk data request
	headers          storage.Headers  // used to fetch the block header when chunk data is ready to be verified
	finishProcessing FinishProcessing // to report a chunk has been processed

	receiptsDB    storage.ExecutionReceipts // used to find executor of the chunk
	retryInterval time.Duration             // determines time in milliseconds for retrying chunk data requests
	maxAttempt    int                       // max time of retries to fetch the chunk data pack for a chunk
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	net module.Network,
	me module.Local,
	verifier network.Engine,
	state protocol.State,
	chunks *Chunks,
	headers storage.Headers,
	retryInterval time.Duration,
	maxAttempt int,
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
		retryInterval: retryInterval,
		maxAttempt:    maxAttempt,
	}

	if maxAttempt == 0 {
		return nil, fmt.Errorf("max retry can not be 0")
	}

	con, err := net.Register(engine.RequestChunks, e)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	e.con = con
	return e, nil
}

func (e *Engine) WithFinishProcessing(finishProcessing FinishProcessing) {
	e.finishProcessing = finishProcessing
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	if e.finishProcessing == nil {
		panic("missing finishProcessing callback in verification match engine")
	}

	delay := time.Duration(0)
	// run a periodic check to retry requesting chunk data packs for chunks that assigned to me.
	// if onTimer takes longer than retryInterval, the next call will be blocked until the previous
	// call has finished.
	// That being said, there won't be two onTimer running in parallel. See test cases for LaunchPeriodically
	e.unit.LaunchPeriodically(e.onTimer, e.retryInterval, delay)
	return e.unit.Ready()
}

// Done terminates the engine and returns a channel that is closed when the termination is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
// Note: this method is required as an Engine implementation,
// however it should not be invoked as match engine requires origin ID of events
// it receives. Use Process method instead.
func (e *Engine) ProcessLocal(event interface{}) error {
	return fmt.Errorf("should not invoke ProcessLocal of Match engine, use Process instead")
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process receives and submits an event to the engine for processing.
// It returns an error so the engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	var err error

	switch resource := event.(type) {
	case *messages.ChunkDataResponse:
		err = e.onChunkDataPack(originID, &resource.ChunkDataPack, &resource.Collection)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	if err != nil {
		// logs the error instead of returning that.
		// returning error would be projected at a higher level by network layer.
		// however, this is an engine-level error, and not network layer error.
		e.log.Debug().Err(err).Msg("engine could not process event successfully")
	}

	return nil
}

// ProcessMyChunk processes the chunk that assigned to me. It should not be blocking since
// multiple workers might be calling it concurrently.
// It skips chunks for sealed blocks.
// It fetches the chunk data pack, once received, verifier engine will be verifying
// Once a chunk has been processed, it will call the FinishProcessing callback to notify
// the chunkconsumer in order to process the next chunk.
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
		e.finishProcessing.Notify(chunkID)
		return
	}

	// skip sealed blocks
	if sealed {
		lg.Debug().Msg("skip sealed chunk")
		e.finishProcessing.Notify(chunkID)
		return
	}

	lg = lg.With().Uint64("height", header.Height).Logger()

	err = e.processChunk(c, header, resultID)

	if err != nil {
		lg.Error().Err(err).Msg("could not process chunk")
		// we report finish processing this chunk even if it failed
		e.finishProcessing.Notify(chunkID)
	} else {
		lg.Info().Msgf("processing chunk")
	}
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

func blockIsSealed(state protocol.State, headers storage.Headers, blockID flow.Identifier) (bool, *flow.Header, error) {
	header, err := headers.ByBlockID(blockID)
	if err != nil {
		return false, nil, fmt.Errorf("could not get block header by ID: %w", err)
	}

	lastSealed, err := state.Sealed().Head()
	if err != nil {
		return false, nil, fmt.Errorf("could not get last sealed: %w", err)
	}

	sealed := header.Height <= lastSealed.Height
	return sealed, header, nil
}

// return agrees and disagrees.
// agrees are executors who made receipt with the same result as the given result id
// disagrees are executors who made receipt with different result than the given result id
func executorsOf(receipts []*flow.ExecutionReceipt, resultID flow.Identifier) ([]flow.Identifier, []flow.Identifier) {
	agrees := make([]flow.Identifier, 0)
	disagrees := make([]flow.Identifier, 0)
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

// onTimer runs periodically, it goes through all pending chunks, and fetches
// its chunk data pack.
// it also retries the chunk data request if the data hasn't been received
// for a while.
func (e *Engine) onTimer() {
	allChunks := e.pendingChunks.All()

	e.log.Debug().Int("total", len(allChunks)).Msg("start processing all pending pendingChunks")

	sealed, err := e.state.Sealed().Head()
	if err != nil {
		e.log.Error().Err(err).Msg("could not get last sealed block")
		return
	}

	allExecutors, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		e.log.Error().Err(err).Msg("could not get executors")
		return
	}

	for _, chunk := range allChunks {
		chunkID := chunk.ID()

		lg := e.log.With().
			Hex("chunk_id", logging.ID(chunkID)).
			Hex("result_id", logging.ID(chunk.ExecutionResultID)).
			Hex("block_id", logging.ID(chunk.Chunk.ChunkBody.BlockID)).
			Uint64("height", chunk.Height).
			Logger()

		// if block has been sealed, then we can finish
		isSealed := chunk.Height <= sealed.Height

		if isSealed {
			removed := e.pendingChunks.Rem(chunkID)
			lg.Info().Bool("removed", removed).Msg("chunk has been sealed, no longer needed")
			continue
		}

		// check if has reached max try
		if !CanTry(e.maxAttempt, chunk) {
			lg.Debug().
				Int("max_attempt", e.maxAttempt).
				Int("actual_attempts", chunk.Attempt).
				Msg("max attempts reached, no longer fetch data pack for chunk")
			continue
		}

		err := e.requestChunkDataPack(chunk, allExecutors)
		if err != nil {
			lg.Warn().Err(err).Msg("could not request chunk data pack")
			continue
		}

		log.Info().Msg("chunk data requested")
	}
}

// requestChunkDataPack request the chunk data pack from the execution node.
// the chunk data pack includes the collection and statecommitments that
// needed to make a VerifiableChunk
func (e *Engine) requestChunkDataPack(c *ChunkStatus, allExecutors flow.IdentityList) error {
	chunkID := c.ID()

	// creates chunk data pack request event
	req := &messages.ChunkDataRequest{
		ChunkID: chunkID,
		Nonce:   rand.Uint64(), // prevent the request from being deduplicated by the receiver
	}

	targetIDs := chooseChunkDataPackTarget(allExecutors, c.Agrees, c.Disagrees)

	// TEMP: skip sending chunk data pack requests to avoid overloading ENs
	// This is OK because RAs are not yet used in the sealing process.
	e.log.Warn().
		Hex("block_id", c.Chunk.BlockID[:]).
		Uint64("chunk_index", c.Chunk.Index).
		Msg("skipping sending chunk data pack request")

	return nil

	// publishes the chunk data request to the network
	err := e.con.Publish(req, targetIDs...)
	if err != nil {
		return fmt.Errorf(
			"could not publish chunk data pack request for chunk (id=%s): %w", chunkID, err)
	}

	exists := e.pendingChunks.IncrementAttempt(c.ID())
	if !exists {
		return fmt.Errorf("chunk is no longer needed to be processed")
	}

	return nil
}

func chooseChunkDataPackTarget(
	allExecutors flow.IdentityList,
	agrees []flow.Identifier,
	disagrees []flow.Identifier,
) []flow.Identifier {
	// if there are enough receipts produced the same result (agrees), we will
	// randomly pick 2 from them
	if len(agrees) >= 2 {
		return allExecutors.Filter(filter.HasNodeID(agrees...)).Sample(2).NodeIDs()
	}

	// since there is at least one agree, then usually, we just need one extra node
	// as a backup.
	// we pick the one extra node randomly from the rest nodes who we haven't received
	// its receipt.
	// In the case where all other ENs has produced different results, then we will only
	// fetch from the one produced the same result (the only agree)
	need := uint(2 - len(agrees))

	nonResponders := allExecutors.Filter(
		filter.Not(filter.HasNodeID(disagrees...))).Sample(need).NodeIDs()

	return append(agrees, nonResponders...)
}

func (e *Engine) onChunkDataPack(
	originID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection,
) error {
	chunkID := chunkDataPack.ChunkID

	// make sure we still need it
	status, exists := e.pendingChunks.ByID(chunkID)
	if !exists {
		return engine.NewInvalidInputErrorf(
			"chunk's data pack is no longer needed, chunk id: %v", chunkID)
	}

	chunk := status.Chunk

	// make sure the chunk data pack is valid
	err := e.validateChunkDataPack(
		chunk, originID, chunkDataPack, collection)
	if err != nil {
		return engine.NewInvalidInputErrorf(
			"invalid chunk data pack for chunk: %v block: %v", chunkID, chunk.BlockID)
	}

	// make sure we won't process duplicated chunk data pack
	removed := e.pendingChunks.Rem(chunkID)
	if !removed {
		return engine.NewInvalidInputErrorf(
			"chunk not found in mempool, likely a race condition, chunk id: %v", chunkID)
	}

	// whenever we removed a chunk from pending chunks, we need to
	// report that the job has been finished eventually
	defer e.finishProcessing.Notify(chunkID)

	resultID := status.ExecutionResultID
	err = e.verifyChunkWithChunkDataPack(chunk, resultID, chunkDataPack, collection)
	if err != nil {
		return fmt.Errorf("could not verify chunk with chunk data pack for result: %v: %w", resultID, err)
	}

	return nil
}

// verifyChunkWithChunkDataPack fetches the result for the executed block, and
// make verifiable chunk data, and pass it to the verifier for verification,
func (e *Engine) verifyChunkWithChunkDataPack(
	chunk *flow.Chunk, resultID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection,
) error {
	header, err := e.headers.ByBlockID(chunk.BlockID)
	if err != nil {
		return fmt.Errorf("could not get block header: %w", err)
	}

	result, err := e.getResultByID(chunk.BlockID, resultID)
	if err != nil {
		return fmt.Errorf("could not get result by id %v: %w", resultID, err)
	}

	vchunk, err := e.makeVerifiableChunkData(
		chunk, header, result, chunkDataPack, collection)

	if err != nil {
		return fmt.Errorf("could not make verifiable chunk data: %w", err)
	}

	err = e.verifier.ProcessLocal(vchunk)
	if err != nil {
		return fmt.Errorf("verifier could not verify chunk: %w", err)
	}

	return nil
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

func (e *Engine) makeVerifiableChunkData(
	chunk *flow.Chunk,
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

// CanTry returns checks the history attempts and determine whether a chunk request
// can be tried again.
func CanTry(maxAttempt int, chunk *ChunkStatus) bool {
	return chunk.Attempt < maxAttempt
}

// IsSystemChunk returns true if `chunkIndex` points to a system chunk in `result`.
// Otherwise, it returns false.
// In the current version, a chunk is a system chunk if it is the last chunk of the
// execution result.
func IsSystemChunk(chunkIndex uint64, result *flow.ExecutionResult) bool {
	return chunkIndex == uint64(len(result.Chunks)-1)
}
