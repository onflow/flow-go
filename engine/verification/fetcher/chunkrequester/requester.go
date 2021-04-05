package chunkrequester

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/rand"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

// Max number of execution nodes being asked for a chunk data pack.
const RequestTargetCount = 2

// Engine is responsible of receiving chunk data pack requests, dispatching it to the execution nodes, receiving
// the requested chunk data pack from execution nodes, and passing it to the registered handler.
type Engine struct {
	log             zerolog.Logger
	unit            *engine.Unit
	handler         fetcher.ChunkDataPackHandler // contains callbacks for handling received chunk data packs.
	retryInterval   time.Duration                // determines time in milliseconds for retrying chunk data requests.
	pendingRequests mempool.ChunkRequests        // used to store all the pending chunks that assigned to this node
	state           protocol.State               // used to check the last sealed height
	con             network.Conduit              // used to send the chunk data request, and receive the response.
}

func New(log zerolog.Logger,
	state protocol.State,
	net module.Network,
	retryInterval time.Duration,
	handler fetcher.ChunkDataPackHandler) (*Engine, error) {

	e := &Engine{
		unit:          engine.NewUnit(),
		log:           log.With().Str("engine", "requester").Logger(),
		retryInterval: retryInterval,
		handler:       handler,
		state:         state,
	}

	if e.handler == nil {
		return nil, fmt.Errorf("missing chunk data pack handler")
	}

	con, err := net.Register(engine.RequestChunks, e)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	e.con = con

	return e, nil
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.log.Fatal().Msg("engine is not supposed to be invoked on SubmitLocal")
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

// Ready initializes the engine and returns a channel that is closed when the initialization is done.
func (e *Engine) Ready() <-chan struct{} {
	delay := time.Duration(0)
	// run a periodic check to retry requesting chunk data packs for chunks that assigned to me.
	// if onTimer takes longer than retryInterval, the next call will be blocked until the previous
	// call has finished.
	// That being said, there won't be two onTimer running in parallel. See test cases for LaunchPeriodically.
	e.unit.LaunchPeriodically(e.onTimer, e.retryInterval, delay)
	return e.unit.Ready()
}

// Done terminates the engine and returns a channel that is closed when the termination is done
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// process receives and submits an event to the engine for processing.
// It returns an error so the engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *messages.ChunkDataResponse:
		e.handleChunkDataPack(originID, &resource.ChunkDataPack, &resource.Collection)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	return nil
}

// handleChunkDataPack sends the received chunk data pack and its collection to the registered handler, and cleans up its request status.
func (e *Engine) handleChunkDataPack(originID flow.Identifier, chunkDataPack *flow.ChunkDataPack, collection *flow.Collection) {
	chunkID := chunkDataPack.ChunkID
	collectionID := collection.ID()
	log := e.log.With().
		Hex("chunk_id", logging.ID(chunkID)).
		Hex("collection_id", logging.ID(collectionID)).
		Logger()

	log.Debug().Msg("chunk data pack received")

	// make sure we still need it.
	_, exists := e.pendingRequests.ByID(chunkID)
	if !exists {
		log.Debug().Msg("chunk data pack is no longer needed, dropped")
		return
	}

	// make sure we won't process duplicated chunk data pack.
	removed := e.pendingRequests.Rem(chunkID)
	if !removed {
		log.Debug().Msg("chunk not found in mempool to be removed, likely a race condition")
		return
	}

	e.handler.HandleChunkDataPack(originID, chunkDataPack, collection)
	log.Info().Msg("successfully sent the chunk data pack to the handler")
}

// Request receives a chunk data pack request and adds it into the pending requests mempool.
func (e *Engine) Request(request *verification.ChunkDataPackRequest, targets flow.IdentityList) {
	status := &verification.ChunkRequestStatus{
		ChunkDataPackRequest: request,
		Targets:              targets,
	}
	added := e.pendingRequests.Add(status)
	e.log.Debug().
		Hex("chunk_id", logging.ID(request.ChunkID)).
		Bool("added_to_pending_requests", added).
		Msg("chunk data pack request arrived")
}

// onTimer should run periodically, it goes through all pending chunks, and requests their chunk data pack.
// It also retries the chunk data request if the data hasn't been received for a while.
func (e *Engine) onTimer() {
	pendingReqs := e.pendingRequests.All()

	e.log.Debug().
		Int("total", len(pendingReqs)).
		Msg("start processing all pending chunk data requests")

	for _, request := range pendingReqs {
		log := e.log.With().
			Hex("chunk_id", logging.ID(request.ID())).
			Uint64("block_height", request.Height).
			Logger()

		// if block has been sealed, then we can finish
		sealed, err := e.blockIsSealed(request.Height)
		if err != nil {
			log.Fatal().Err(err).Msg("could not determine whether block has been sealed")
		}

		if sealed {
			removed := e.pendingRequests.Rem(request.ID())
			log.Info().
				Bool("removed", removed).
				Msg("drops requesting chunk of a seald block")
			continue
		}

		err = e.requestChunkDataPack(request)
		if err != nil {
			log.Warn().Err(err).Msg("could not request chunk data pack")
			continue
		}

		log.Info().Msg("chunk data pack requested")
	}
}

// blockIsSealed returns true if the block at specified height is sealed.
func (e Engine) blockIsSealed(height uint64) (bool, error) {
	lastSealed, err := e.state.Sealed().Head()
	if err != nil {
		return false, fmt.Errorf("could not get last sealed: %w", err)
	}

	sealed := height <= lastSealed.Height
	return sealed, nil
}

// requestChunkDataPack dispatches request for the chunk data pack to the execution nodes.
func (e *Engine) requestChunkDataPack(status *verification.ChunkRequestStatus) error {
	// creates chunk data pack request event
	req := &messages.ChunkDataRequest{
		ChunkID: status.ChunkID,
		Nonce:   rand.Uint64(), // prevent the request from being deduplicated by the receiver
	}

	// publishes the chunk data request to the network
	targetIDs := status.SampleTargets(RequestTargetCount)
	err := e.con.Publish(req, targetIDs...)
	if err != nil {
		return fmt.Errorf("could not publish chunk data pack request for chunk (id=%s): %w", status.ChunkID, err)
	}

	exists := e.pendingRequests.IncrementAttempt(status.ID())
	if !exists {
		return fmt.Errorf("chunk is no longer needed to be processed")
	}

	return nil
}
