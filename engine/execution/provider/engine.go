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
	DefaultChunkDataPackRequestWorker   = 100
	DefaultChunkDataPackProcessInterval = 10 * time.Millisecond
	DefaultChunkDataPackQueryTimeout    = 10 * time.Second
	DefaultChunkDataPackDeliveryTimeout = 10 * time.Second
)

// An Engine provides means of accessing data about execution state and broadcasts execution receipts to nodes in the network.
// Also generates and saves execution receipts
type Engine struct {
	component.Component
	cm *component.ComponentManager

	log                        zerolog.Logger
	tracer                     module.Tracer
	receiptCon                 network.Conduit
	state                      protocol.State
	execState                  state.ReadOnlyExecutionState
	chunksConduit              network.Conduit
	metrics                    module.ExecutionMetrics
	chdpRequestsQueue          mempool.ChunkDataPackRequestQueue
	checkAuthorizedAtBlock     func(blockID flow.Identifier) (bool, error)
	chdpQueryTimeout           time.Duration
	chdpDeliveryTimeout        time.Duration
	chdpRequestProcessInterval time.Duration
}

func New(
	logger zerolog.Logger,
	tracer module.Tracer,
	net network.Network,
	state protocol.State,
	execState state.ReadOnlyExecutionState,
	metrics module.ExecutionMetrics,
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error),
	chunkDataPackRequestQueue mempool.ChunkDataPackRequestQueue,
	chdpQueryTimeout time.Duration,
	chdpDeliveryTimeout time.Duration,
	chdpRequestProcessInterval time.Duration,
	chdpRequestWorkers uint,
) (*Engine, error) {

	log := logger.With().Str("engine", "receipts").Logger()

	engine := Engine{
		log:                        log,
		tracer:                     tracer,
		state:                      state,
		execState:                  execState,
		metrics:                    metrics,
		checkAuthorizedAtBlock:     checkAuthorizedAtBlock,
		chdpQueryTimeout:           chdpQueryTimeout,
		chdpDeliveryTimeout:        chdpDeliveryTimeout,
		chdpRequestProcessInterval: chdpRequestProcessInterval,
		chdpRequestsQueue:          chunkDataPackRequestQueue,
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
	for i := uint(0); i < chdpRequestWorkers; i++ {
		cm.AddWorker(engine.processChunkDataPackRequestWorker)
	}

	engine.cm = cm.Build()
	engine.Component = engine.cm

	return &engine, nil
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.cm.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.cm.Done()
}

func (e *Engine) processChunkDataPackRequestWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	ticker := time.NewTicker(e.chdpRequestProcessInterval)
	e.log.Debug().Msg("process chunk data pack request worker started")

	for {
		select {
		case <-ticker.C:
			request, exists := e.chdpRequestsQueue.Pop()
			if !exists {
				e.log.Trace().Msg("worker did not find any chunk data pack request, passing this cycle")
				continue
			}
			lg := e.log.With().
				Hex("chunk_id", logging.ID(request.ChunkId)).
				Hex("origin_id", logging.ID(request.RequesterId)).Logger()
			lg.Trace().Msg("chunk data pack request picked for processing")
			e.onChunkDataRequest(request)
			lg.Trace().Msg("chunk data pack request processed")
		case <-ctx.Done():
			e.log.Trace().Msg("processing chunk data pack request worker terminated")
			return
		}
	}
}

func (e *Engine) Process(channel channels.Channel, originID flow.Identifier, event interface{}) error {
	return e.process(originID, event)
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch v := event.(type) {
	case *messages.ChunkDataRequest:
		pushed := e.chdpRequestsQueue.Push(v.ChunkID, originID)
		e.log.Info().
			Hex("chunk_id", logging.ID(v.ChunkID)).
			Hex("origin_id", logging.ID(originID)).
			Bool("enqueued", pushed).
			Msg("chunk data pack request received")
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
	return nil
}

// onChunkDataRequest receives a request for the chunk data pack associated with chunkID from the
// requester `originID`. If such a chunk data pack is available in the execution state, it is sent to the
// requester.
func (e *Engine) onChunkDataRequest(request *mempool.ChunkDataPackRequest) {
	processStart := time.Now()

	lg := e.log.With().
		Hex("origin_id", logging.ID(request.RequesterId)).
		Hex("chunk_id", logging.ID(request.ChunkId)).
		Logger()

	lg.Info().Msg("received chunk data pack request")

	// increases collector metric
	e.metrics.ChunkDataPackRequested()

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

	_, err = e.ensureAuthorized(chunkDataPack.ChunkID, request.RequesterId)
	if err != nil {
		lg.Error().
			Err(err).
			Msg("could not verify authorization of identity of chunk data pack request, dropping it")
		return
	}

	response := &messages.ChunkDataResponse{
		ChunkDataPack: *chunkDataPack,
		Nonce:         rand.Uint64(),
	}

	sinceProcess := time.Since(processStart)
	lg = lg.With().Dur("sinceProcess", sinceProcess).Logger()

	if sinceProcess > e.chdpQueryTimeout {
		lg.Warn().Msgf("chunk data pack query takes longer than %v secs", e.chdpQueryTimeout.Seconds())
	}

	lg.Info().Msg("chunk data pack response lunched to dispatch")

	// sends requested chunk data pack to the requester

	deliveryStart := time.Now()

	err = e.chunksConduit.Unicast(response, request.RequesterId)

	sinceDeliver := time.Since(deliveryStart)
	lg = lg.With().Dur("since_deliver", sinceDeliver).Logger()

	if sinceDeliver > e.chdpDeliveryTimeout {
		lg.Warn().Msgf("chunk data pack response delivery takes longer than %v secs", e.chdpDeliveryTimeout.Seconds())
	}

	if err != nil {
		lg.Warn().
			Err(err).
			Msg("could not send requested chunk data pack to origin ID")
		return
	}

	if response.ChunkDataPack.Collection != nil {
		// logging collection id of non-system chunks.
		// A system chunk has both the collection and collection id set to nil.
		lg = lg.With().
			Hex("collection_id", logging.ID(response.ChunkDataPack.Collection.ID())).
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
