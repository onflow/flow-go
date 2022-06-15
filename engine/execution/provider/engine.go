package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type ProviderEngine interface {
	network.Engine
	BroadcastExecutionReceipt(context.Context, *flow.ExecutionReceipt) error
}

// An Engine provides means of accessing data about execution state and broadcasts execution receipts to nodes in the network.
// Also generates and saves execution receipts
type Engine struct {
	unit                   *engine.Unit
	log                    zerolog.Logger
	tracer                 module.Tracer
	receiptCon             network.Conduit
	state                  protocol.State
	execState              state.ReadOnlyExecutionState
	me                     module.Local
	chunksConduit          network.Conduit
	metrics                module.ExecutionMetrics
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error)
	chdpQueryTimeout       time.Duration
	chdpDeliveryTimeout    time.Duration
}

func New(
	logger zerolog.Logger,
	tracer module.Tracer,
	net network.Network,
	state protocol.State,
	me module.Local,
	execState state.ReadOnlyExecutionState,
	metrics module.ExecutionMetrics,
	checkAuthorizedAtBlock func(blockID flow.Identifier) (bool, error),
	chdpQueryTimeout uint,
	chdpDeliveryTimeout uint,
) (*Engine, error) {

	log := logger.With().Str("engine", "receipts").Logger()

	eng := Engine{
		unit:                   engine.NewUnit(),
		log:                    log,
		tracer:                 tracer,
		state:                  state,
		me:                     me,
		execState:              execState,
		metrics:                metrics,
		checkAuthorizedAtBlock: checkAuthorizedAtBlock,
		chdpQueryTimeout:       time.Duration(chdpQueryTimeout) * time.Second,
		chdpDeliveryTimeout:    time.Duration(chdpDeliveryTimeout) * time.Second,
	}

	var err error

	eng.receiptCon, err = net.Register(engine.PushReceipts, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register receipt provider engine: %w", err)
	}

	chunksConduit, err := net.Register(engine.ProvideChunks, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register chunk data pack provider engine: %w", err)
	}
	eng.chunksConduit = chunksConduit

	return &eng, nil
}

func (e *Engine) SubmitLocal(event interface{}) {
	e.unit.Launch(func() {
		err := e.ProcessLocal(event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

func (e *Engine) Submit(channel network.Channel, originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(channel, originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(e.me.NodeID(), event)
	})
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) Process(channel network.Channel, originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	ctx := context.Background()
	switch v := event.(type) {
	case *messages.ChunkDataRequest:
		e.onChunkDataRequest(ctx, originID, v)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}

	return nil
}

// onChunkDataRequest receives a request for the chunk data pack associated with chunkID from the
// requester `originID`. If such a chunk data pack is available in the execution state, it is sent to the
// requester.
func (e *Engine) onChunkDataRequest(
	ctx context.Context,
	originID flow.Identifier,
	req *messages.ChunkDataRequest,
) {

	processStart := time.Now()

	// extracts list of verifier nodes id
	chunkID := req.ChunkID

	lg := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Logger()

	lg.Info().Msg("received chunk data pack request")

	// increases collector metric
	e.metrics.ChunkDataPackRequested()

	chunkDataPack, err := e.execState.ChunkDataPackByChunkID(ctx, chunkID)
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

	_, err = e.ensureAuthorized(chunkDataPack.ChunkID, originID)
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

	lg.Debug().Msg("chunk data pack response lunched to dispatch")

	// sends requested chunk data pack to the requester
	e.unit.Launch(func() {
		deliveryStart := time.Now()

		err := e.chunksConduit.Unicast(response, originID)

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
	})
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
	defer span.Finish()

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
