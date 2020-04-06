package provider

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/sync"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ProviderEngine interface {
	network.Engine
	BroadcastExecutionReceipt(*flow.ExecutionReceipt) error
}

// An Engine provides means of accessing data about execution state and broadcasts execution receipts to nodes in the network.
// Also generates and saves execution receipts
type Engine struct {
	unit          *engine.Unit
	log           zerolog.Logger
	receiptCon    network.Conduit
	state         protocol.ReadOnlyState
	execState     state.ReadOnlyExecutionState
	me            module.Local
	execStateCon  network.Conduit
	chunksConduit network.Conduit
	stateSync     sync.StateSynchronizer
	syncCon   network.Conduit
}

func New(
	logger zerolog.Logger,
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
		state:     state,
		me:        me,
		execState: execState,
		stateSync: stateSync,
	}

	var err error

	eng.receiptCon, err = net.Register(engine.ExecutionReceiptProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register receipt provider engine")
	}

	eng.execStateCon, err = net.Register(engine.ExecutionStateProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution state provider engine")
	}

	chunksConduit, err := net.Register(engine.ChunkDataPackProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register chunk data pack provider engine")
	}
	eng.chunksConduit = chunksConduit

	//eng.syncCon, err = net.Register(engine.ExecutionSync, &eng)
	//if err != nil {
	//	return nil, errors.Wrap(err, "could not register execution sync engine")
	//}

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
		var err error
		switch v := event.(type) {
		case *messages.ExecutionStateRequest:
			err = e.onExecutionStateRequest(originID, v)
		case *messages.ChunkDataPackRequest:
			err = e.handleChunkDataPackRequest(originID, v.ChunkID)
		case *messages.ExecutionStateDelta:
			return e.onExecutionStateDelta(originID, v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
}

func (e *Engine) onExecutionStateRequest(originID flow.Identifier, req *messages.ExecutionStateRequest) error {
	chunkID := req.ChunkID

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("received execution state request")

	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", id, err)
	}

	if id.Role != flow.RoleVerification {
		return fmt.Errorf("invalid role for requesting execution state: %s", id.Role)
	}

	registers, err := e.execState.GetChunkRegisters(chunkID)
	if err != nil {
		return fmt.Errorf("could not retrieve chunk state (id=%s): %w", chunkID, err)
	}

	msg := &messages.ExecutionStateResponse{State: flow.ChunkState{
		ChunkID:   chunkID,
		Registers: registers,
	}}

	err = e.execStateCon.Submit(msg, id.NodeID)
	if err != nil {
		return fmt.Errorf("could not submit response for chunk state (id=%s): %w", chunkID, err)
	}

	return nil
}


func (e *Engine) onExecutionStateDelta(originID flow.Identifier, req *messages.ExecutionStateDelta) error {
	// TODO: apply delta to store
	// Does this belong in this engine? Does it matter if we are removing the engines anyways?
	return nil
}

// handleChunkDataPackRequest receives a request for the chunk data pack associated with chunkID from the
// requester `originID`. If such a chunk data pack is available in the execution state, it is sent to the
// requester.
func (e *Engine) handleChunkDataPackRequest(originID flow.Identifier, chunkID flow.Identifier) error {
	// extracts list of verifier nodes id
	//
	// TODO state extraction should be done based on block references
	// https://github.com/dapperlabs/flow-go/issues/2787
	origin, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("invalid origin id (%s): %w", origin, err)
	}

	// only verifier nodes are allowed to request chunk data packs
	if origin.Role != flow.RoleVerification {
		return fmt.Errorf("invalid role for receiving collection: %s", origin.Role)
	}

	cdp, err := e.execState.ChunkDataPackByChunkID(chunkID)
	if err != nil {
		return fmt.Errorf("could not retrieve chunk ID (%s): %w", origin, err)
	}

	response := &messages.ChunkDataPackResponse{
		Data: *cdp,
	}

	// sends requested chunk data pack to the requester
	err = e.chunksConduit.Submit(response, originID)
	if err != nil {
		return fmt.Errorf("could not send requested chunk data pack to (%s): %w", origin, err)
	}

	return nil
}

func (e *Engine) BroadcastExecutionReceipt(receipt *flow.ExecutionReceipt) error {

	e.log.Debug().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("broadcasting execution receipt")

	identities, err := e.state.Final().Identities(filter.HasRole(flow.RoleConsensus, flow.RoleVerification))
	if err != nil {
		return fmt.Errorf("could not get consensus and verification identities: %w", err)
	}

	nodeIDs := identities.NodeIDs()
	err = e.receiptCon.Submit(receipt, nodeIDs...)
	if err != nil {
		return fmt.Errorf("could not submit execution receipts: %w", err)
	}

	return nil
}
