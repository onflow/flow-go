package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

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
	unit          *engine.Unit
	log           zerolog.Logger
	tracer        module.Tracer
	receiptCon    network.Conduit
	state         protocol.State
	execState     state.ReadOnlyExecutionState
	me            module.Local
	chunksConduit network.Conduit
	metrics       module.ExecutionMetrics
}

func New(
	logger zerolog.Logger,
	tracer module.Tracer,
	net module.Network,
	state protocol.State,
	me module.Local,
	execState state.ReadOnlyExecutionState,
	metrics module.ExecutionMetrics,
) (*Engine, error) {

	log := logger.With().Str("engine", "receipts").Logger()

	eng := Engine{
		unit:      engine.NewUnit(),
		log:       log,
		tracer:    tracer,
		state:     state,
		me:        me,
		execState: execState,
		metrics:   metrics,
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
	e.Submit(e.me.NodeID(), event)
}

func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
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

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	ctx := context.Background()
	switch v := event.(type) {
	case *messages.ChunkDataRequest:
		err := e.onChunkDataRequest(ctx, originID, v)
		if err != nil {
			return fmt.Errorf("could not answer chunk data request: %w", err)
		}
		return err
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onChunkDataRequest receives a request for the chunk data pack associated with chunkID from the
// requester `originID`. If such a chunk data pack is available in the execution state, it is sent to the
// requester.
func (e *Engine) onChunkDataRequest(
	ctx context.Context,
	originID flow.Identifier,
	req *messages.ChunkDataRequest,
) error {

	// extracts list of verifier nodes id
	chunkID := req.ChunkID

	log := e.log.With().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Logger()

	log.Debug().Msg("received chunk data pack request")

	// increases collector metric
	e.metrics.ChunkDataPackRequested()

	cdp, err := e.execState.ChunkDataPackByChunkID(ctx, chunkID)
	// we might be behind when we don't have the requested chunk.
	// if this happen, log it and return nil
	if errors.Is(err, storage.ErrNotFound) {
		log.Warn().Msg("chunk not found")
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not retrieve chunk ID (%s): %w", originID, err)
	}

	origin, err := e.ensureStaked(cdp.ChunkID, originID)
	if err != nil {
		return err
	}

	var collection flow.Collection
	if cdp.CollectionID != flow.ZeroID {
		// retrieves collection of non-zero chunks
		coll, err := e.execState.GetCollection(cdp.CollectionID)
		if err != nil {
			return fmt.Errorf("cannot retrieve collection %x for chunk %x: %w", cdp.CollectionID, cdp.ChunkID, err)
		}
		collection = *coll
	}

	response := &messages.ChunkDataResponse{
		ChunkDataPack: *cdp,
		Nonce:         rand.Uint64(),
		Collection:    collection,
	}

	// sends requested chunk data pack to the requester
	err = e.chunksConduit.Unicast(response, originID)
	if err != nil {
		return fmt.Errorf("could not send requested chunk data pack to (%s): %w", origin, err)
	}

	log.Debug().
		Hex("collection_id", logging.ID(response.Collection.ID())).
		Msg("chunk data pack request successfully replied")

	return nil
}

func (e *Engine) ensureStaked(chunkID flow.Identifier, originID flow.Identifier) (*flow.Identity, error) {
	blockID, err := e.execState.GetBlockIDByChunkID(chunkID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("cannot find blockID corresponding to chunk data pack: %w", err)
	}

	origin, err := e.state.AtBlockID(blockID).Identity(originID)
	if err != nil {
		return nil, engine.NewInvalidInputErrorf("invalid origin id (%s): %w", origin, err)
	}

	// only verifier nodes are allowed to request chunk data packs
	if origin.Role != flow.RoleVerification {
		return nil, engine.NewInvalidInputErrorf("invalid role for receiving collection: %s", origin.Role)
	}

	if origin.Stake == 0 {
		return nil, engine.NewInvalidInputErrorf("node %s is not staked for the epoch corresponding to the requested chunk data pack", origin.NodeID)
	}
	return origin, nil
}

func (e *Engine) BroadcastExecutionReceipt(ctx context.Context, receipt *flow.ExecutionReceipt) error {
	finalState, ok := receipt.ExecutionResult.FinalStateCommitment()
	if !ok {
		return fmt.Errorf("could not get final state: no chunks found")
	}

	span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEBroadcastExecutionReceipt)
	defer span.Finish()

	e.log.Debug().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("final_state", finalState).
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
