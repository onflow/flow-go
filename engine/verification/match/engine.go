package match

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	chunkmodule "github.com/dapperlabs/flow-go/module/chunks"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine takes processable execution results, finds the chunks the are assigned to me, fetches
// the chunk data pack from execution nodes, and passes verifiable chunks to Verifier engine
type Engine struct {
	unit          *engine.Unit
	log           zerolog.Logger
	metrics       module.VerificationMetrics
	tracer        module.Tracer
	me            module.Local
	results       mempool.PendingResults // used to store all the execution results along with their senders
	verifier      network.Engine         // the verifier engine
	assignment    chunkmodule.Assignment // used to determine chunks this node needs to verify
	state         protocol.State         // used to verify the request origin
	pendingChunks *Chunks                // used to store all the pending chunks that assigned to this node
	con           network.Conduit        // used to send the chunk data request
	headers       storage.Headers        // used to fetch the block header when chunk data is ready to be verified
	retryInterval time.Duration          // determines time in milliseconds for retrying chunk data requests
	maxAttempt    int                    // max time of retries to fetch the chunk data pack for a chunk
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	net module.Network,
	me module.Local,
	results mempool.PendingResults,
	verifier network.Engine,
	assignment chunkmodule.Assignment,
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
		log:           log.With().Str("engine", "match").Logger(),
		me:            me,
		results:       results,
		verifier:      verifier,
		assignment:    assignment,
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

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
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
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
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
	case *flow.ExecutionResult:
		err = e.handleExecutionResult(originID, resource)
	case *messages.ChunkDataResponse:
		err = e.handleChunkDataPack(originID, &resource.ChunkDataPack, &resource.Collection)
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

// handleExecutionResult takes a execution result and finds chunks that are assigned to this
// verification node and adds them to the pending chunk list to be processed.
// It stores the result in memory, in order to check if a chunk still needs to be processed.
func (e *Engine) handleExecutionResult(originID flow.Identifier, result *flow.ExecutionResult) error {
	resultID := result.ID()
	blockID := result.ExecutionResultBody.BlockID

	// metrics
	//
	// traces running time
	span := e.tracer.StartSpan(resultID, trace.VERProcessExecutionResult)
	span.SetTag("execution_result_id", resultID)
	ctx := opentracing.ContextWithSpan(context.Background(), span)
	childSpan, ctx := e.tracer.StartSpanFromContext(ctx, trace.VERMatchHandleExecutionResult)
	defer childSpan.Finish()
	// monitoring: increases number of received execution results
	e.metrics.OnExecutionResultReceived()

	log := e.log.With().
		Hex("originID", logging.ID(originID)).
		Hex("result_id", logging.ID(resultID)).
		Hex("block_id", logging.ID(blockID)).
		Int("total_chunks", len(result.Chunks)).
		Logger()

	log.Info().Msg("execution result arrived")

	pendingResult := &flow.PendingResult{
		ExecutorID:      originID,
		ExecutionResult: result,
	}

	added := e.results.Add(pendingResult)

	if !added {
		// drops processing execution result if it has already
		// been added to results mempool
		log.Debug().
			Msg("duplicate execution result dropped")
		return nil
	}

	// different execution results can be chunked in parallel
	chunks, err := e.myChunkAssignments(ctx, result)
	if err != nil {
		return fmt.Errorf("could not find my chunk assignments: %w", err)
	}

	log.Info().
		Int("total_assigned_chunks", len(chunks)).
		Msg("chunk assignment done")

	// handles the assigned chunks
	for _, chunk := range chunks {
		e.handleChunk(chunk, resultID, originID)
	}

	log.Debug().
		Int("total_assigned_chunks", len(chunks)).
		Uint("total_pending_chunks", e.pendingChunks.Size()).
		Msg("finish processing execution result")
	return nil
}

// myChunkAssignments returns the list of chunks in the chunk list that this verification node
// is assigned to.
func (e *Engine) myChunkAssignments(ctx context.Context, result *flow.ExecutionResult) (flow.ChunkList, error) {
	var span opentracing.Span
	span, ctx = e.tracer.StartSpanFromContext(ctx, trace.VERMatchMyChunkAssignments)
	defer span.Finish()

	verifiers, err := e.state.Final().
		Identities(filter.HasRole(flow.RoleVerification))
	if err != nil {
		return nil, fmt.Errorf("could not load verifier node IDs: %w", err)
	}

	mine, err := e.assignment.MyChunks(e.me.NodeID(), verifiers, result)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

// DEPRECATED: Moved all logic relating to assignment to /module/chunks/assignment.go
func myAssignements(ctx context.Context, assigner module.ChunkAssigner, myID flow.Identifier,
	verifiers flow.IdentityList, result *flow.ExecutionResult) (flow.ChunkList, error) {

	// The randomness of the assignment is taken from the result.
	// TODO: taking the randomness from the random beacon, which is included in it's next block
	rng, err := utils.NewChunkAssignmentRNG(result)
	if err != nil {
		return nil, fmt.Errorf("could not generate random generator: %w", err)
	}

	assignment, err := assigner.Assign(verifiers, result.Chunks, rng)
	if err != nil {
		return nil, fmt.Errorf("could not create chunk assignment %w", err)
	}

	// indices of chunks assigned to this node
	chunkIndices := assignment.ByNodeID(myID)

	// mine keeps the list of chunks assigned to this node
	mine := make(flow.ChunkList, 0, len(chunkIndices))
	for _, index := range chunkIndices {
		chunk, ok := result.Chunks.ByIndex(index)
		if !ok {
			return nil, fmt.Errorf("chunk out of range requested: %v", index)
		}

		mine = append(mine, chunk)
	}

	return mine, nil
}

// onTimer runs periodically, it goes through all pending chunks, and fetches
// its chunk data pack.
// it also retries the chunk data request if the data hasn't been received
// for a while.
func (e *Engine) onTimer() {
	allChunks := e.pendingChunks.All()

	now := time.Now()
	e.log.Debug().Int("total", len(allChunks)).Msg("start processing all pending pendingChunks")
	defer e.log.Debug().
		Int("processed", len(allChunks)-int(e.pendingChunks.Size())).
		Uint("left", e.pendingChunks.Size()).
		Dur("duration", time.Since(now)).
		Msg("finish processing all pending pendingChunks")

	for _, chunk := range allChunks {
		cid := chunk.ID()

		log := e.log.With().
			Hex("chunk_id", cid[:]).
			Hex("result_id", chunk.ExecutionResultID[:]).
			Logger()

		// check if has reached max try
		if !CanTry(e.maxAttempt, chunk) {
			log.Debug().
				Int("max_attempt", e.maxAttempt).
				Int("actual_attempts", chunk.Attempt).
				Msg("max attempts reached, chunk is not longer retried")
			continue
		}

		exists := e.results.Has(chunk.ExecutionResultID)
		// if execution result has been removed, no need to request
		// the chunk data any more.
		if !exists {
			e.pendingChunks.Rem(cid)
			log.Debug().Msg("remove chunk since execution result no longer exists")
			continue
		}

		err := e.requestChunkDataPack(chunk)
		if err != nil {
			log.Warn().Msg("could not request chunk data pack")
			continue
		}

		exists = e.pendingChunks.IncrementAttempt(cid)
		if !exists {
			log.Debug().Msg("skip if chunk no longer exists")
			continue
		}

		log.Info().Msg("chunk data requested")
	}

}

// requestChunkDataPack request the chunk data pack from the execution node.
// the chunk data pack includes the collection and statecommitments that
// needed to make a VerifiableChunk
func (e *Engine) requestChunkDataPack(c *ChunkStatus) error {
	chunkID := c.ID()

	execNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution nodes identities: %w", err)
	}

	// request from the exeuctor plus another random execution node as a backup
	nodes := execNodes.Filter(filter.Not(filter.HasNodeID(c.ExecutorID, e.me.NodeID()))).Sample(1).NodeIDs()
	nodes = append(nodes, c.ExecutorID)

	req := &messages.ChunkDataRequest{
		ChunkID: chunkID,
		Nonce:   rand.Uint64(), // prevent the request from being deduplicated by the receiver
	}

	err = e.con.Submit(req, nodes...)
	if err != nil {
		return fmt.Errorf("could not submit chunk data pack request for chunk (id=%s): %w", chunkID, err)
	}

	return nil
}

// handleChunk handles a chunk by creating a
// chunk status for the chunk and adds it to the pending chunks mempool to be processed by onTimer
func (e *Engine) handleChunk(chunk *flow.Chunk, resultID flow.Identifier, executorID flow.Identifier) {

	status := NewChunkStatus(chunk, resultID, executorID)
	added := e.pendingChunks.Add(status)
	if !added {
		e.log.Debug().
			Hex("chunk_id", logging.ID(chunk.ID())).
			Hex("result_id", logging.ID(status.ExecutionResultID)).
			Msg("could not add chunk status to pendingChunks mempool")
		return
	}
	e.log.Debug().
		Hex("chunk_id", logging.ID(chunk.ID())).
		Hex("result_id", logging.ID(status.ExecutionResultID)).
		Msg("chunk marked assigned to this verification node")
}

// handleChunkDataPack receives a chunk data pack, verifies its origin ID, pull other data to make a
// VerifiableChunk, and pass it to the verifier engine to verify
func (e *Engine) handleChunkDataPack(originID flow.Identifier,
	chunkDataPack *flow.ChunkDataPack,
	collection *flow.Collection) error {
	start := time.Now()

	chunkID := chunkDataPack.ChunkID

	log := e.log.With().
		Hex("executor_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).Logger()
	log.Info().Msg("chunk data pack received")

	// monitoring: increments number of received chunk data packs
	e.metrics.OnChunkDataPackReceived()

	// check origin is from a execution node
	// TODO check the origin is a node that we requested before
	sender, err := e.state.Final().Identity(originID)
	if errors.Is(err, storage.ErrNotFound) {
		return engine.NewInvalidInputErrorf("origin is unstaked: %v", originID)
	}

	if err != nil {
		return fmt.Errorf("could not find identity for chunkID %v: %w", chunkID, err)
	}

	if sender.Role != flow.RoleExecution {
		return engine.NewInvalidInputError("receives chunk data pack from a non-execution node")
	}

	status, exists := e.pendingChunks.ByID(chunkID)
	if !exists {
		return engine.NewInvalidInputErrorf("chunk does not exist, chunkID: %v", chunkID)
	}

	// TODO: verify the collection ID matches with the collection guarantee in the block payload

	// remove first to ensure concurrency issue
	removed := e.pendingChunks.Rem(chunkDataPack.ChunkID)
	if !removed {
		return engine.NewInvalidInputErrorf("chunk has not been removed, chunkID: %v", chunkID)
	}

	resultID := status.ExecutionResultID

	if span, ok := e.tracer.GetSpan(resultID, trace.VERProcessExecutionResult); ok {
		childSpan := e.tracer.StartSpanFromParent(span, trace.VERMatchHandleChunkDataPack, opentracing.StartTime(start))
		defer childSpan.Finish()
	}

	result, exists := e.results.ByID(resultID)
	if !exists {
		// result no longer exists
		return engine.NewInvalidInputErrorf("execution result ID no longer exist: %v, for chunkID :%v", status.ExecutionResultID, chunkID)
	}

	// computes the end state of the chunk
	var isSystemChunk bool
	var endState flow.StateCommitment
	if int(status.Chunk.Index) == len(result.ExecutionResult.Chunks)-1 {
		// last chunk in a result is the system chunk and takes final state commitment
		isSystemChunk = true
		endState = result.ExecutionResult.FinalStateCommit
	} else {
		// any chunk except last takes the subsequent chunk's start state
		isSystemChunk = false
		endState = result.ExecutionResult.Chunks[status.Chunk.Index+1].StartState
	}

	// matches the chunk as a non-system chunk
	err = e.matchChunk(
		isSystemChunk,
		status.Chunk,
		result.ExecutionResult,
		collection,
		chunkDataPack,
		endState)

	blockID := result.ExecutionResult.BlockID
	if err != nil {
		log.Warn().
			Err(err).
			Hex("block_id", logging.ID(blockID)).
			Hex("result_id", logging.ID(resultID)).
			Msg("failed to match the chunk")
	} else {
		log.Info().
			Hex("block_id", logging.ID(blockID)).
			Hex("result_id", logging.ID(resultID)).
			Msg("chunk successfully matched")
	}

	return nil
}

// matchChunk performs the last step in matching pipeline for a chunk.
// It captures the chunk into a verifiable chunk and submits it to the
// verifier engine.
func (e *Engine) matchChunk(
	isSystemChunk bool,
	chunk *flow.Chunk,
	result *flow.ExecutionResult,
	collection *flow.Collection,
	chunkDataPack *flow.ChunkDataPack,
	endState flow.StateCommitment) error {

	blockID := result.ExecutionResultBody.BlockID

	// header must exist in storage
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not find block header: %w", err)
	}

	// creates a verifiable chunk for assigned chunk
	vchunk := &verification.VerifiableChunkData{
		IsSystemChunk: isSystemChunk,
		Chunk:         chunk,
		Header:        header,
		Result:        result,
		Collection:    collection,
		ChunkDataPack: chunkDataPack,
		EndState:      endState,
	}

	e.unit.Launch(func() {
		err = e.verifier.ProcessLocal(vchunk)
		// metrics: increases number of verifiable chunks sent
		e.metrics.OnVerifiableChunkSent()
	})

	return err
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
	if chunkIndex == uint64(len(result.Chunks)-1) {
		return true
	} else {
		return false
	}
}
