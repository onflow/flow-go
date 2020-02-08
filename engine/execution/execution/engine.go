package execution

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/language"
	"github.com/dapperlabs/flow-go/language/encoding"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ExecutionEngine interface {
	network.Engine
	ExecuteScript([]byte, *flow.Header, *state.View) ([]byte, error)
}

// Engine manages execution of transactions.
type Engine struct {
	unit       *engine.Unit
	log        zerolog.Logger
	me         module.Local
	protoState protocol.State
	provider   network.Engine
	vm         virtualmachine.VirtualMachine
	executor   executor.BlockExecutor
}

func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	protoState protocol.State,
	receipts network.Engine,
	vm virtualmachine.VirtualMachine,
) (*Engine, error) {
	log := logger.With().Str("engine", "execution").Logger()

	executor := executor.NewBlockExecutor(vm)

	e := Engine{
		unit:       engine.NewUnit(),
		log:        log,
		me:         me,
		protoState: protoState,
		provider:   receipts,
		vm:         vm,
		executor:   executor,
	}

	var err error

	_, err = net.Register(engine.ExecutionExecution, &e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register execution engine")
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

func (e *Engine) ExecuteScript(script []byte, blockHeader *flow.Header, view *state.View) ([]byte, error) {

	result, err := e.vm.NewBlockContext(blockHeader).ExecuteScript(view, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if !result.Succeeded() {
		return nil, fmt.Errorf("failed to execute script: %w", result.Error)
	}

	value, err := language.ConvertValue(result.Value)
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
	case *execution.ComputationOrder:
		return e.onCompleteBlock(originID, ev.Block, ev.View, ev.StartState)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onCompleteBlock is triggered when this engine receives a new block.
//
// This function passes the complete block to the block executor and
// then submits the result to the provider engine.
func (e *Engine) onCompleteBlock(originID flow.Identifier, block *execution.CompleteBlock, view *state.View, startState flow.StateCommitment) error {
	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote request to execute complete block [%x]", block.Block.ID())
	}

	result, err := e.executor.ExecuteBlock(block, view, startState)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return fmt.Errorf("failed to execute block: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(result.CompleteBlock.Block)).
		Msg("computed block result")

	// submit execution result to provider engine
	e.provider.SubmitLocal(result)

	return nil
}
