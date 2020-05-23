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

type Engine struct {
	unit          *engine.Unit
	log           zerolog.Logger
	me            module.Local
	results       Results
	verifier      network.Engine
	assigner      module.ChunkAssigner // used to determine chunks this node needs to verify
	state         protocol.State
	chunks        *Chunks
	con           network.Conduit
	headers       storage.Headers
	retryInterval time.Duration // determines time in milliseconds for retrying chunk data requests

}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	results Results,
	verifier network.Engine,
	assigner module.ChunkAssigner,
	state protocol.State,
	chunks *Chunks,
	con network.Conduit,
	headers storage.Headers,
	retryInterval time.Duration,
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
		con:           con,
		headers:       headers,
		retryInterval: retryInterval,
	}

	_, err := net.Register(engine.ChunkDataPackProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	return e, nil
}

// Ready initializes the engine and returns a channel that is closed when the initialization is done
func (e *Engine) Ready() <-chan struct{} {
	delay := time.Duration(0)
	e.unit.LaunchPeriodically(e.OnTimer, e.retryInterval, delay)
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
	case *messages.ChunkDataPackResponse:
		return e.handleChunkDataPack(originID, &resource.Data)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) handleExecutionResult(originID flow.Identifier, r *flow.ExecutionResult) error {
	result := &Result{
		ExecutorID:      originID,
		ExecutionResult: r,
	}

	added := e.results.Add(result)

	// is new exeuction result
	if !added {
		return nil
	}

	// chunking can be run in parallel
	chunks, err := e.myChunkAssignments(result.ExecutionResult)
	if err != nil {
		return fmt.Errorf("could not find my chunk assignments: %w", err)
	}

	for _, chunk := range chunks {
		status := NewChunkStatus(chunk, result.ExecutionResult.ID(), result.ExecutorID)
		added := e.chunks.Add(status)
		if !added {
			//TODO: log
			e.log.Debug().Msg("chunk has already been added")
			continue
		}
	}
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

	// TODO pull up caching of chunk assignments to here
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

func (e *Engine) OnTimer() {
	allChunks := e.chunks.All()

	for _, chunk := range allChunks {
		id := chunk.ID()
		c, exists := e.chunks.ByID(id)
		// double check
		if !exists {
			// deleted
			continue
		}

		err := e.requestChunkDataPack(c)
		if err != nil {
			e.log.Warn().Msg("could not request chunk data pack")
		}
	}
}

func (e *Engine) requestChunkDataPack(c *ChunkStatus) error {
	chunkID := c.Chunk.ID()

	execNodes, err := e.state.Final().Identities(filter.HasRole(flow.RoleExecution))
	if err != nil {
		return fmt.Errorf("could not load execution nodes identities: %w", err)
	}

	nodes := execNodes.Filter(filter.Not(filter.HasNodeID(c.ExecutorID, e.me.NodeID()))).Sample(1).NodeIDs()
	nodes = append(nodes, c.ExecutorID)

	req := &messages.ChunkDataPackRequest{
		ChunkID: chunkID,
		Nonce:   rand.Uint64(),
	}

	err = e.con.Submit(req, nodes...)
	if err != nil {
		return fmt.Errorf("could not submit chunk data pack request for chunk (id=%s): %w", chunkID, err)
	}

	return nil
}

// handleChunkDataPack receives a chunk data pack, verifies its origin ID, and stores that in the mempool
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack) error {
	// check origin is from a exeuction node
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_data_pack_id", logging.Entity(chunkDataPack)).
		Msg("chunk data pack received")

	status, exists := e.chunks.ByID(chunkDataPack.ChunkID)
	if !exists {
		return nil
	}

	// delete first to ensure concurrency issue
	deleted := e.chunks.Rem(chunkDataPack.ChunkID)
	if !deleted {
		// no longer exists
		// TODO
		return nil
	}

	result, exists := e.results.ByID(status.ExecutionResultID)
	if !exists {
		// result no longer exists
		return nil
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
		ChunkDataPack: chunkDataPack,
	}

	e.unit.Launch(func() {
		e.verifier.Submit(status.ExecutorID, vchunk)
	})

	return nil
}
