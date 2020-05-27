package match

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/utils"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
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
	me            module.Local
	results       module.PendingResults // used to store all the execution results along with their senders
	verifier      network.Engine        // the verifier engine
	assigner      module.ChunkAssigner  // used to determine chunks this node needs to verify
	state         protocol.State        // used to verify the request origin
	chunks        *Chunks               // used to store all the chunks that assigned to me
	con           network.Conduit       // used to send the chunk data request
	headers       storage.Headers       // used to fetch the block header when chunk data is ready to be verified
	retryInterval time.Duration         // determines time in milliseconds for retrying chunk data requests
	maxAttempt    int                   // max time of retries to fetch the chunk data pack for a chunk
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	results module.PendingResults,
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
		log:           log,
		me:            me,
		results:       results,
		verifier:      verifier,
		assigner:      assigner,
		state:         state,
		chunks:        chunks,
		headers:       headers,
		retryInterval: retryInterval,
		maxAttempt:    maxAttempt,
	}

	if maxAttempt == 0 {
		return nil, fmt.Errorf("max retry can not be 0")
	}

	con, err := net.Register(engine.ChunkDataPackProvider, e)
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
			e.log.Error().Err(err).Msg("could not process submitted event")
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
	log := e.log.With().
		Hex("originID", originID[:]).
		Hex("execution_result_id", logging.ID(r.ID())).
		Logger()

	log.Debug().Msg("start processing execution result")

	result := &flow.PendingResult{
		ExecutorID:      originID,
		ExecutionResult: r,
	}

	added := e.results.Add(result)

	// if a execution result has been added before, then don't process
	// this result.
	if !added {
		return fmt.Errorf("execution result has been added before: %v", r.ID())
	}

	// different execution results can be chunked in parallel
	chunks, err := e.myChunkAssignments(result.ExecutionResult)
	if err != nil {
		return fmt.Errorf("could not find my chunk assignments: %w", err)
	}

	// add each chunk to a pending list to be processed by onTimer
	for _, chunk := range chunks {
		status := NewChunkStatus(chunk, result.ExecutionResult.ID(), result.ExecutorID)
		_ = e.chunks.Add(status)
	}

	log.Debug().
		Int("chunks", len(chunks)).
		Uint("pending", e.chunks.Size()).
		Msg("finish processing execution result")
	return nil
}

// myUningestedChunks returns the list of chunks in the chunk list that this verifier node
// is assigned to, and are not ingested yet. A chunk is ingested once a verifiable chunk is
// formed out of it and is passed to verify engine
func (e *Engine) myChunkAssignments(result *flow.ExecutionResult) (flow.ChunkList, error) {
	verifiers, err := e.state.Final().
		Identities(filter.HasRole(flow.RoleVerification))
	if err != nil {
		return nil, fmt.Errorf("could not load verifier node IDs: %w", err)
	}

	mine, err := myAssignements(e.assigner, e.me.NodeID(), verifiers, result)
	if err != nil {
		return nil, fmt.Errorf("could not determine my assignments: %w", err)
	}

	return mine, nil
}

func myAssignements(assigner module.ChunkAssigner, myID flow.Identifier, verifiers flow.IdentityList, result *flow.ExecutionResult) (flow.ChunkList, error) {
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
	allChunks := e.chunks.All()

	now := time.Now()
	e.log.Debug().Int("total", len(allChunks)).Msg("start processing all pending chunks")
	defer e.log.Debug().
		Int("processed", len(allChunks)-int(e.chunks.Size())).
		Uint("left", e.chunks.Size()).
		Dur("duration", time.Since(now)).
		Msg("finish processing all pending chunks")

	for _, chunk := range allChunks {
		cid := chunk.ID()

		log := e.log.With().
			Hex("chunk_id", cid[:]).
			Hex("result_id", chunk.ExecutionResultID[:]).
			Logger()

		// check if has reached max try
		if !CanTry(e.maxAttempt, chunk) {
			e.chunks.Rem(cid)
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
			e.chunks.Rem(cid)
			log.Debug().Msg("remove chunk since execution result no longer exists")
			continue
		}

		exists = e.chunks.IncrementAttempt(cid)
		if !exists {
			log.Debug().Msg("skip if chunk no longer exists")
			continue
		}

		err := e.requestChunkDataPack(chunk)
		if err != nil {
			log.Warn().Msg("could not request chunk data pack")
			continue
		}

		log.Debug().Msg("chunk data requested")
	}

}

// request the chunk data pack from the execution node.
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
	log := e.log.With().
		Hex("executor_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).Logger()
	log.Info().Msg("chunk data pack received")

	// check origin is from a execution node
	sender, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not find identity: %w", err)
	}

	if sender.Role != flow.RoleExecution {
		return fmt.Errorf("receives chunk data pack from a non-execution node")
	}

	status, exists := e.chunks.ByID(chunkDataPack.ChunkID)
	if !exists {
		return fmt.Errorf("chunk does not exist, chunkID: %v", chunkDataPack.ChunkID)
	}

	// TODO: verify the collection ID matches with the collection guarantee in the block payload

	// remove first to ensure concurrency issue
	removed := e.chunks.Rem(chunkDataPack.ChunkID)
	if !removed {
		return fmt.Errorf("chunk has been removed, chunkID: %v", chunkDataPack.ChunkID)
	}

	result, exists := e.results.ByID(status.ExecutionResultID)
	if !exists {
		// result no longer exists
		return fmt.Errorf("execution result ID no longer exist: %v", status.ExecutionResultID)
	}

	// header must exist in storage
	header, err := e.headers.ByBlockID(result.ExecutionResult.ExecutionResultBody.BlockID)
	if err != nil {
		return fmt.Errorf("could not find block header: %w", err)
	}

	// creates a verifiable chunk for assigned chunk
	vchunk := &verification.VerifiableChunkData{
		Chunk:         status.Chunk,
		Header:        header,
		Result:        result.ExecutionResult,
		Collection:    collection,
		ChunkDataPack: chunkDataPack,
	}

	e.unit.Launch(func() {
		err := e.verifier.ProcessLocal(vchunk)
		if err != nil {
			log.Warn().Err(err).Msg("failed to verify chunk")
			return
		}
		log.Info().Msg("chunk verified")
	})

	return nil
}

// CanTry returns checks the history attempts and determine whether a chunk request
// can be tried again.
func CanTry(maxAttempt int, chunk *ChunkStatus) bool {
	return chunk.Attempt < maxAttempt
}
