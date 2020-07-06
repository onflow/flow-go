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
	assigner      module.ChunkAssigner   // used to determine chunks this node needs to verify
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
	assigner module.ChunkAssigner,
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
		assigner:      assigner,
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
	switch resource := event.(type) {
	case *flow.ExecutionResult:
		return e.handleExecutionResult(originID, resource)
	case *messages.ChunkDataResponse:
		return e.handleChunkDataPack(originID, &resource.ChunkDataPack, &resource.Collection)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// handleExecutionResult takes a execution result and find chunks that are assigned to me, and add them to
// the pending chunk list to be processed.
// It stores the result in memory, in order to check if a chunk still needs to be processed.
func (e *Engine) handleExecutionResult(originID flow.Identifier, r *flow.ExecutionResult) error {
	span := e.tracer.StartSpan(r.ID(), trace.VERProcessExecutionResult)
	span.SetTag("execution_result_id", r.ID())
	ctx := opentracing.ContextWithSpan(context.Background(), span)
	childSpan, ctx := e.tracer.StartSpanFromContext(ctx, trace.VERMatchHandleExecutionResult)
	defer childSpan.Finish()

	log := e.log.With().
		Hex("originID", originID[:]).
		Hex("execution_result_id", logging.ID(r.ID())).
		Logger()
	// monitoring: increases number of received execution results
	e.metrics.OnExecutionResultReceived()

	log.Debug().Msg("start processing execution result")

	result := &flow.PendingResult{
		ExecutorID:      originID,
		ExecutionResult: r,
	}

	added := e.results.Add(result)

	// if a execution result has been added before, then don't process
	// this result.
	if !added {
		log.Debug().
			Hex("result_id", logging.ID(r.ID())).
			Msg("execution result has been added")
		return nil
	}

	// different execution results can be chunked in parallel
	chunks, err := e.myChunkAssignments(ctx, result.ExecutionResult)
	if err != nil {
		return fmt.Errorf("could not find my chunk assignments: %w", err)
	}

	// add each chunk to a pending list to be processed by onTimer
	for _, chunk := range chunks {
		status := NewChunkStatus(chunk, result.ExecutionResult.ID(), result.ExecutorID)
		added = e.pendingChunks.Add(status)
		if !added {
			log.Debug().
				Int("pendingChunks", len(chunks)).
				Msg("could not add chunk status to pendingChunks mempool")
		}
	}

	log.Debug().
		Int("pendingChunks", len(chunks)).
		Uint("pending", e.pendingChunks.Size()).
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

	mine, err := myAssignements(ctx, e.assigner, e.me.NodeID(), verifiers, result)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

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
			// TODO not to drop max reach, but to ignore it
			e.pendingChunks.Rem(cid)
			log.Debug().
				Int("max_attempt", e.maxAttempt).
				Int("actual_attempts", chunk.Attempt).
				Msg("max attampts reached")
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

		log.Debug().Msg("chunk data requested")
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

// handleChunkDataPack receives a chunk data pack, verifies its origin ID, pull other data to make a
// VerifiableChunk, and pass it to the verifier engine to verify
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) error {
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

	blockID := result.ExecutionResult.ExecutionResultBody.BlockID
	// header must exist in storage
	header, err := e.headers.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not find block header: %w for chunkID: %v", err, chunkID)
	}

	// computes the end state of the chunk
	var endState flow.StateCommitment
	if int(status.Chunk.Index) == len(result.ExecutionResult.Chunks)-1 {
		// last chunk in result takes final state commitment
		endState = result.ExecutionResult.FinalStateCommit
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.ExecutionResult.Chunks[status.Chunk.Index+1].StartState
	}

	// creates a verifiable chunk for assigned chunk
	// TODO: replace with VerifiableChunk
	vchunk := &verification.VerifiableChunkData{
		Chunk:         status.Chunk,
		Header:        header,
		Result:        result.ExecutionResult,
		Collection:    collection,
		ChunkDataPack: chunkDataPack,
		EndState:      endState,
	}

	e.unit.Launch(func() {
		err := e.verifier.ProcessLocal(vchunk)
		log = log.With().
			Hex("block_id", blockID[:]).
			Hex("result_id", resultID[:]).
			Logger()
		if err != nil {
			log.Warn().Err(err).Msg("failed to verify chunk")
			return
		}
		log.Info().Msg("chunk verified")
		// metrics: increases number of verifiable chunks sent
		e.metrics.OnVerifiableChunkSent()
	})

	return nil
}

// CanTry returns checks the history attempts and determine whether a chunk request
// can be tried again.
func CanTry(maxAttempt int, chunk *ChunkStatus) bool {
	return chunk.Attempt < maxAttempt
}
