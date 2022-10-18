package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type ProviderEngine interface {
	network.MessageProcessor
	BroadcastExecutionReceipt(context.Context, *flow.ExecutionReceipt) error
}

const (
	// DefaultChunkDataPackRequestWorker is the default number of concurrent workers processing chunk data pack requests on
	// execution nodes.
	DefaultChunkDataPackRequestWorker = 100
	// DefaultChunkDataPackQueryTimeout is the default timeout value for querying a chunk data pack from storage.
	DefaultChunkDataPackQueryTimeout = 10 * time.Second
	// DefaultChunkDataPackDeliveryTimeout is the default timeout value for delivery of a chunk data pack to a verification
	// node.
	DefaultChunkDataPackDeliveryTimeout = 10 * time.Second
)

// An Engine provides means of accessing data about execution state and broadcasts execution receipts to nodes in the network.
// Also generates and saves execution receipts
type Engine struct {
	component.Component
	cm *component.ComponentManager

	log                    zerolog.Logger
	tracer                 module.Tracer
	receiptCon             network.Conduit
	state                  protocol.State
	execState              state.ReadOnlyExecutionState
	chunksConduit          network.Conduit
	metrics                module.ExecutionMetrics
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error)
	chdpRequestHandler     *engine.MessageHandler
	chdpRequestQueue       engine.MessageStore

	// buffered channel for ChunkDataRequest workers to pick
	// requests and process.
	chdpRequestChannel chan *mempool.ChunkDataPackRequest

	// timeout for delivery of a chunk data pack response in the network.
	chunkDataPackDeliveryTimeout time.Duration
	// timeout for querying chunk data pack through database.
	chunkDataPackQueryTimeout time.Duration
}

func New(
	logger zerolog.Logger,
	tracer module.Tracer,
	net network.Network,
	state protocol.State,
	execState state.ReadOnlyExecutionState,
	metrics module.ExecutionMetrics,
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error),
	chunkDataPackRequestQueue engine.MessageStore,
	chdpRequestWorkers uint,
	chunkDataPackQueryTimeout time.Duration,
	chunkDataPackDeliveryTimeout time.Duration,
) (*Engine, error) {

	log := logger.With().Str("engine", "receipts").Logger()

	handler := engine.NewMessageHandler(
		log,
		engine.NewNotifier(),
		engine.Pattern{
			// Match is called on every new message coming to this engine.
			// Provider enigne only expects ChunkDataRequests.
			// Other message types are discarded by Match.
			Match: func(message *engine.Message) bool {
				chdpReq, ok := message.Payload.(*messages.ChunkDataRequest)
				if ok {
					log.Info().
						Hex("chunk_id", logging.ID(chdpReq.ChunkID)).
						Hex("requester_id", logging.ID(message.OriginID)).
						Msg("chunk data pack request received")
				}
				return ok
			},
			// Map is called on messages that are Match(ed) successfully, i.e.,
			// ChunkDataRequests.
			// It replaces the payload of message with requested chunk id.
			Map: func(message *engine.Message) (*engine.Message, bool) {
				chdpReq := message.Payload.(*messages.ChunkDataRequest)
				return &engine.Message{
					OriginID: message.OriginID,
					Payload:  chdpReq.ChunkID,
				}, true
			},
			Store: chunkDataPackRequestQueue,
		})

	engine := Engine{
		log:                          log,
		tracer:                       tracer,
		state:                        state,
		execState:                    execState,
		metrics:                      metrics,
		checkAuthorizedAtBlock:       checkAuthorizedAtBlock,
		chdpRequestHandler:           handler,
		chdpRequestQueue:             chunkDataPackRequestQueue,
		chdpRequestChannel:           make(chan *mempool.ChunkDataPackRequest, chdpRequestWorkers),
		chunkDataPackDeliveryTimeout: chunkDataPackDeliveryTimeout,
		chunkDataPackQueryTimeout:    chunkDataPackQueryTimeout,
	}

	var err error

	engine.receiptCon, err = net.Register(channels.PushReceipts, &engine)
	if err != nil {
		return nil, fmt.Errorf("could not register receipt provider engine: %w", err)
	}

	chunksConduit, err := net.Register(channels.ProvideChunks, &engine)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	engine.chunksConduit = chunksConduit

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(engine.processQueuedChunkDataPackRequestsShovelerWorker)
	for i := uint(0); i < chdpRequestWorkers; i++ {
		cm.AddWorker(engine.processChunkDataPackRequestWorker)
	}

	engine.cm = cm.Build()
	engine.Component = engine.cm

	return &engine, nil
}

// processQueuedChunkDataPackRequestsShovelerWorker is constantly listening on the MessageHandler for ChunkDataRequests,
// and pushes new ChunkDataRequests into the request channel to be picked by workers.
func (e *Engine) processQueuedChunkDataPackRequestsShovelerWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	e.log.Debug().Msg("process chunk data pack request shovller worker started")

	for {
		select {
		case <-e.chdpRequestHandler.GetNotifier():
			// there is at list a single chunk data pack request queued up.
			e.shovelChunkDataPackRequests()
		case <-ctx.Done():
			// close the internal channel, the workers will drain the channel before exiting
			close(e.chdpRequestChannel)
			e.log.Trace().Msg("processing chunk data pack request worker terminated")
			return
		}
	}
}

// shovelChunkDataPackRequests is a blocking method that reads all queued ChunkDataRequests till the queue gets empty.
// Each ChunkDataRequest is processed by a single concurrent worker. However, there are limited number of such workers.
// If there is no worker available for a request, the method blocks till one is available.
func (e *Engine) shovelChunkDataPackRequests() {
	for {
		msg, ok := e.chdpRequestQueue.Get()
		if !ok {
			// no more requests, return
			return
		}

		request := &mempool.ChunkDataPackRequest{
			RequesterId: msg.OriginID,
			ChunkId:     msg.Payload.(flow.Identifier),
		}
		lg := e.log.With().
			Hex("chunk_id", logging.ID(request.ChunkId)).
			Hex("origin_id", logging.ID(request.RequesterId)).Logger()

		lg.Trace().Msg("shovller is queuing chunk data pack request for processing")
		e.chdpRequestChannel <- request
		lg.Trace().Msg("shovller queued up chunk data pack request for processing")
	}
}

// processChunkDataPackRequestWorker encapsulates the logic of a single (concurrent) worker that picks a
// ChunkDataRequest from this engine's queue and processes it.
func (e *Engine) processChunkDataPackRequestWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		request, ok := <-e.chdpRequestChannel
		if !ok {
			e.log.Trace().Msg("processing chunk data pack request worker terminated")
			return
		}
		lg := e.log.With().
			Hex("chunk_id", logging.ID(request.ChunkId)).
			Hex("origin_id", logging.ID(request.RequesterId)).Logger()
		lg.Trace().Msg("worker picked up chunk data pack request for processing")
		e.onChunkDataRequest(request)
		lg.Trace().Msg("worker finished chunk data pack processing")
	}
}

func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.cm.ShutdownSignal():
		e.log.Warn().Msgf("received message from %x after shut down", originID)
		return nil
	default:
	}

	err := e.chdpRequestHandler.Process(originID, event)
	if err != nil {
		if engine.IsIncompatibleInputTypeError(err) {
			e.log.Warn().Msgf("%v delivered unsupported message %T through %v", originID, event, channel)
			return nil
		}
		return fmt.Errorf("unexpected error while processing engine message: %w", err)
	}

	return nil
}

// onChunkDataRequest receives a request for a chunk data pack,
// and if such a chunk data pack is available in the execution state, it is sent to the requester node.
// TODO improve error handling https://github.com/dapperlabs/flow-go/issues/6363
func (e *Engine) onChunkDataRequest(request *mempool.ChunkDataPackRequest) {
	processStartTime := time.Now()

	lg := e.log.With().
		Hex("origin_id", logging.ID(request.RequesterId)).
		Hex("chunk_id", logging.ID(request.ChunkId)).
		Logger()
	lg.Info().Msg("started processing chunk data pack request")

	// increases collector metric
	e.metrics.ChunkDataPackRequestProcessed()
	chunkDataPack, err := e.execState.ChunkDataPackByChunkID(request.ChunkId)

	// we might be behind when we don't have the requested chunk.
	// if this happen, log it and return nil
	if errors.Is(err, storage.ErrNotFound) {
		lg.Warn().
			Err(err).
			Msg("chunk data pack not found, execution node may be behind")
		return
	}
	if err != nil {
		lg.Error().
			Err(err).
			Msg("could not retrieve chunk ID from storage")
		return
	}

	queryTime := time.Since(processStartTime)
	lg = lg.With().Dur("query_time", queryTime).Logger()
	if queryTime > e.chunkDataPackQueryTimeout {
		lg.Warn().
			Dur("query_timout", e.chunkDataPackQueryTimeout).
			Msg("chunk data pack query takes longer than expected timeout")
	}

	_, err = e.ensureAuthorized(chunkDataPack.ChunkID, request.RequesterId)
	if err != nil {
		lg.Error().
			Err(err).
			Msg("could not verify authorization of identity of chunk data pack request")
		return
	}

	e.deliverChunkDataResponse(chunkDataPack, request.RequesterId)
}

// deliverChunkDataResponse delivers chunk data pack to the requester through network.
func (e *Engine) deliverChunkDataResponse(chunkDataPack *flow.ChunkDataPack, requesterId flow.Identifier) {
	lg := e.log.With().
		Hex("origin_id", logging.ID(requesterId)).
		Hex("chunk_id", logging.ID(chunkDataPack.ChunkID)).
		Logger()
	lg.Info().Msg("sending chunk data pack response")

	// sends requested chunk data pack to the requester
	deliveryStartTime := time.Now()

	response := &messages.ChunkDataResponse{
		ChunkDataPack: *chunkDataPack,
		Nonce:         rand.Uint64(),
	}

	err := e.chunksConduit.Unicast(response, requesterId)
	if err != nil {
		lg.Warn().
			Err(err).
			Msg("could not send requested chunk data pack to requester")
		return
	}

	deliveryTime := time.Since(deliveryStartTime)
	lg = lg.With().Dur("delivery_time", deliveryTime).Logger()
	if deliveryTime > e.chunkDataPackDeliveryTimeout {
		lg.Warn().
			Dur("delivery_timout", e.chunkDataPackDeliveryTimeout).
			Msg("chunk data pack delivery takes longer than expected timeout")
	}

	if chunkDataPack.Collection != nil {
		// logging collection id of non-system chunks.
		// A system chunk has both the collection and collection id set to nil.
		lg = lg.With().
			Hex("collection_id", logging.ID(chunkDataPack.Collection.ID())).
			Logger()
	}
	lg.Info().Msg("chunk data pack request successfully replied")
}

func (e *Engine) ensureAuthorized(chunkID flow.Identifier, originID flow.Identifier) (*flow.Identity, error) {
	blockID, err := e.execState.GetBlockIDByChunkID(chunkID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("cannot find blockID corresponding to chunk data pack: %w", err)
	}

	authorizedAt, err := e.checkAuthorizedAtBlock(blockID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("cannot check block staking status: %w", err)
	}
	if !authorizedAt {
		return nil, engine.NewInvalidInputErrorf("this node is not authorized at the block (%s) corresponding to chunk data pack (%s)", blockID.String(), chunkID.String())
	}

	origin, err := e.state.AtBlockID(blockID).Identity(originID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("invalid origin id (%s): %w", origin, err)
	}

	// only verifier nodes are allowed to request chunk data packs
	if origin.Role != flow.RoleVerification {
		return nil, engine.NewInvalidInputErrorf("invalid role for receiving collection: %s", origin.Role)
	}

	if origin.Weight == 0 {
		return nil, engine.NewInvalidInputErrorf("node %s has zero weight at the block (%s) corresponding to chunk data pack (%s)", originID, blockID.String(), chunkID.String())
	}
	return origin, nil
}

func (e *Engine) BroadcastExecutionReceipt(ctx context.Context, receipt *flow.ExecutionReceipt) error {
	finalState, err := receipt.ExecutionResult.FinalStateCommitment()
	if err != nil {
		return fmt.Errorf("could not get final state: %w", err)
	}

	span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEBroadcastExecutionReceipt)
	defer span.End()

	e.log.Debug().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("final_state", finalState[:]).
		Msg("broadcasting execution receipt")

	identities, err := e.state.Final().Identities(filter.HasRole(flow.RoleAccess, flow.RoleConsensus,
		flow.RoleVerification))
	if err != nil {
		return fmt.Errorf("could not get consensus and verification identities: %w", err)
	}

	err = e.receiptCon.Publish(receipt, identities.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit execution receipts: %w", err)
	}

	return nil
}
