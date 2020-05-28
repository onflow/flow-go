package provider

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
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
	state         protocol.ReadOnlyState
	execState     state.ReadOnlyExecutionState
	me            module.Local
	chunksConduit network.Conduit
}

func New(
	logger zerolog.Logger,
	tracer module.Tracer,
	net module.Network,
	state protocol.ReadOnlyState,
	me module.Local,
	execState state.ReadOnlyExecutionState,
	stateSync sync.StateSynchronizer,
) (*Engine, error) {

	log := logger.With().Str("engine", "receipts").Logger()

	eng := Engine{
		unit:      engine.NewUnit(),
		log:       log,
		tracer:    tracer,
		state:     state,
		me:        me,
		execState: execState,
	}

	var err error

	eng.receiptCon, err = net.Register(engine.ExecutionReceiptProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register receipt provider engine")
	}

	chunksConduit, err := net.Register(engine.ChunkDataPackProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register chunk data pack provider engine")
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
			e.log.Error().Err(err).Msg("could not process submitted event")
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
		ctx := context.Background()
		var err error
		switch v := event.(type) {
		case *messages.ChunkDataRequest:
			err = e.onChunkDataRequest(ctx, originID, v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
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

	origin, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", origin, err)
	}

	// only verifier nodes are allowed to request chunk data packs
	if origin.Role != flow.RoleVerification {
		return fmt.Errorf("invalid role for receiving collection: %s", origin.Role)
	}

	cdp, err := e.execState.ChunkDataPackByChunkID(ctx, chunkID)
	if err != nil {
		return fmt.Errorf("could not retrieve chunk ID (%s): %w", origin, err)
	}

	collection, err := e.execState.GetCollection(cdp.CollectionID)
	if err != nil {
		return fmt.Errorf("cannot retrieve collection %x for chunk %x: %w", cdp.CollectionID, cdp.ChunkID, err)
	}

	response := &messages.ChunkDataResponse{
		ChunkDataPack: *cdp,
		Nonce:         rand.Uint64(),
		Collection:    *collection,
	}

	log.Debug().Msg("sending chunk data pack response")

	// sends requested chunk data pack to the requester
	err = e.chunksConduit.Submit(response, originID)
	if err != nil {
		return fmt.Errorf("could not send requested chunk data pack to (%s): %w", origin, err)
	}

	return nil
}

func (e *Engine) BroadcastExecutionReceipt(ctx context.Context, receipt *flow.ExecutionReceipt) error {

	span, _ := e.tracer.StartSpanFromContext(ctx, trace.EXEBroadcastExecutionReceipt)
	defer span.Finish()

	e.log.Debug().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Hex("final_state", receipt.ExecutionResult.FinalStateCommit).
		Msg("broadcasting execution receipt")

	identities, err := e.state.Final().Identities(filter.HasRole(flow.RoleConsensus, flow.RoleVerification))
	if err != nil {
		return fmt.Errorf("could not get consensus and verification identities: %w", err)
	}

	err = e.receiptCon.Submit(receipt, identities.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit execution receipts: %w", err)
	}

	return nil
}
