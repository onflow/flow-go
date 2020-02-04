package execution

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/language/runtime/encoding"
	"github.com/dapperlabs/flow-go/language/runtime/values"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine manages execution of transactions.
type Engine struct {
	unit             *engine.Unit
	log              zerolog.Logger
	me               module.Local
	protoState       protocol.State
	execState        state.ExecutionState
	execStateConduit network.Conduit
	receipts         network.Engine
	vm               virtualmachine.VirtualMachine
	executor         executor.BlockExecutor
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	protoState protocol.State,
	execState state.ExecutionState,
	receipts network.Engine,
	vm virtualmachine.VirtualMachine,
) (*Engine, error) {
	log := logger.With().Str("engine", "execution").Logger()

	executor := executor.NewBlockExecutor(vm, execState)

	e := Engine{
		unit:       engine.NewUnit(),
		log:        log,
		me:         me,
		protoState: protoState,
		execState:  execState,
		receipts:   receipts,
		vm:         vm,
		executor:   executor,
	}

	var err error

	_, err = net.Register(engine.ExecutionExecution, &e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution engine")
	}

	e.execStateConduit, err = net.Register(engine.ExecutionStateProvider, &e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution state engine")
	}

	return &e, nil
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

func (e *Engine) ExecuteScript(script []byte) ([]byte, error) {
	// TODO: replace with latest sealed block
	block, err := e.protoState.Final().Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	stateCommit, err := e.execState.StateCommitmentByBlockID(block.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block state commitment: %w", err)
	}

	blockView := e.execState.NewView(stateCommit)

	result, err := e.vm.NewBlockContext(block).ExecuteScript(blockView, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if !result.Succeeded() {
		return nil, fmt.Errorf("failed to execute script: %w", result.Error)
	}

	value, err := values.Convert(result.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to export runtime value: %w", err)
	}

	encodedValue, err := encoding.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	return encodedValue, nil
}

// process processes events for the execution engine on the execution node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *execution.CompleteBlock:
		return e.onCompleteBlock(originID, ev)
	case *messages.ExecutionStateRequest:
		return e.onExecutionStateRequest(originID, ev)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onCompleteBlock is triggered when this engine receives a new block.
//
// This function passes the complete block to the block executor and
// then submits the result to the receipts engine.
func (e *Engine) onCompleteBlock(originID flow.Identifier, block *execution.CompleteBlock) error {
	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote request to execute complete block [%x]", block.Block.ID())
	}

	result, err := e.executor.ExecuteBlock(block)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return fmt.Errorf("failed to execute block: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Hex("result_id", logging.Entity(result)).
		Msg("computed block result")

	// submit execution result to receipt engine
	e.receipts.SubmitLocal(result)

	return nil
}

func (e *Engine) onExecutionStateRequest(originID flow.Identifier, req *messages.ExecutionStateRequest) error {
	chunkID := req.ChunkID

	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("received execution state request")

	id, err := e.protoState.Final().Identity(originID)
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

	err = e.execStateConduit.Submit(msg, id.NodeID)
	if err != nil {
		return fmt.Errorf("could not submit response for chunk state (id=%s): %w", chunkID, err)
	}
	e.log.Info().
		Hex("origin_id", logging.ID(originID)).
		Hex("chunk_id", logging.ID(chunkID)).
		Msg("responded to execution state request")

	return nil
}
