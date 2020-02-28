package provider

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine provides means of accessing data about execution state and broadcasts execution receipts to nodes in the network.
// Also generates and saves execution receipts
type Engine struct {
	unit         *engine.Unit
	log          zerolog.Logger
	receiptCon   network.Conduit
	state        protocol.State
	execState    state.ExecutionState
	me           module.Local
	execStateCon network.Conduit
}

func New(logger zerolog.Logger, net module.Network, state protocol.State, me module.Local, execState state.ExecutionState) (*Engine, error) {
	log := logger.With().Str("engine", "receipts").Logger()

	eng := Engine{
		unit:      engine.NewUnit(),
		log:       log,
		state:     state,
		me:        me,
		execState: execState,
	}

	receiptCon, err := net.Register(engine.ExecutionReceiptProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register receipt provider engine")
	}
	eng.receiptCon = receiptCon

	eng.execStateCon, err = net.Register(engine.ExecutionStateProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution state provider engine")
	}

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
		case *execution.ComputationResult:
			err = e.onExecutionResult(originID, v)
		case *messages.ExecutionStateRequest:
			return e.onExecutionStateRequest(originID, v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
}

func (e *Engine) onExecutionResult(originID flow.Identifier, result *execution.ComputationResult) error {

	e.log.Debug().
		Hex("block_id", logging.ID(result.CompleteBlock.Block.ID())).
		Msg("received execution result")

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote request to submit execution result for block [%x]", result.CompleteBlock.Block.ID())
	}

	chunks := make([]*flow.Chunk, len(result.StateViews))

	startState := result.StartState
	var endState flow.StateCommitment

	for i, view := range result.StateViews {
		// TODO - Should the deltas be applied to a particular state?
		// Not important now, but might become important once we produce proofs

		endState, err := e.execState.CommitDelta(view.Delta())
		if err != nil {
			return fmt.Errorf("failed to apply chunk delta: %w", err)
		}
		//
		chunk := generateChunk(i, startState, endState)
		//
		chunkHeader := generateChunkHeader(chunk, view.Reads())
		//
		err = e.execState.PersistChunkHeader(chunkHeader)
		if err != nil {
			return fmt.Errorf("failed to save chunk header: %w", err)
		}
		//
		chunks[i] = chunk
		startState = endState
	}

	executionResult, err := e.generateExecutionResultForBlock(result.CompleteBlock, chunks, endState)
	if err != nil {
		return fmt.Errorf("could not generate execution result: %w", err)
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: *executionResult,
		// TODO: include SPoCKs
		Spocks: nil,
		// TODO: sign execution receipt
		ExecutorSignature: nil,
		ExecutorID:        e.me.NodeID(),
	}

	err = e.broadcastExecutionReceipt(receipt)
	if err != nil {
		return fmt.Errorf("could not broadcast receipt: %w", err)
	}

	err = e.execState.PersistStateCommitment(result.CompleteBlock.Block.ID(), endState)
	if err != nil {
		return fmt.Errorf("failed to store state commitment: %w", err)
	}

	return nil
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

func (e *Engine) broadcastExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	e.log.Debug().
		Hex("block_id", logging.ID(receipt.ExecutionResult.BlockID)).
		Hex("receipt_id", logging.Entity(receipt)).
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

// generateExecutionResultForBlock creates a new execution result for a block from
// the provided chunk results.
func (e *Engine) generateExecutionResultForBlock(
	block *execution.CompleteBlock,
	chunks []*flow.Chunk,
	endState flow.StateCommitment,
) (*flow.ExecutionResult, error) {

	previousErID, err := e.execState.GetExecutionResultID(block.Block.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get previous execution result ID: %w", err)
	}

	er := &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: previousErID,
			BlockID:          block.Block.ID(),
			FinalStateCommit: endState,
			Chunks:           chunks,
		},
	}

	err = e.execState.PersistExecutionResult(block.Block.ID(), *er)
	if err != nil {
		return nil, fmt.Errorf("could not persist execution result: %w", err)
	}

	return er, nil
}

// generateChunk creates a chunk from the provided execution data.
func generateChunk(colIndex int, startState, endState flow.StateCommitment) *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: uint(colIndex),
			StartState:      startState,
			// TODO: include event collection hash
			EventCollection: flow.ZeroID,
			// TODO: record gas used
			TotalComputationUsed: 0,
			// TODO: record number of txs
			NumberOfTransactions: 0,
		},
		Index:    0,
		EndState: endState,
	}
}

// generateChunkHeader creates a chunk header from the provided chunk and register IDs.
func generateChunkHeader(
	chunk *flow.Chunk,
	registerIDs []flow.RegisterID,
) *flow.ChunkHeader {
	//reads := make([]flow.RegisterID, len(registerIDs))
	//
	//for i, registerID := range registerIDs {
	//	reads[i] = flow.RegisterID(registerID)
	//}

	return &flow.ChunkHeader{
		ChunkID:     chunk.ID(),
		StartState:  chunk.StartState,
		RegisterIDs: registerIDs,
	}
}
