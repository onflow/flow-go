package execution

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
)

// Engine manages execution of transactions
type Engine struct {
	unit     *engine.Unit
	log      zerolog.Logger
	con      network.Conduit
	me       module.Local
	receipts network.Engine
	executor executor.BlockExecutor
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	receipts network.Engine,
	executor executor.BlockExecutor,
) (*Engine, error) {

	e := Engine{
		unit:     engine.NewUnit(),
		log:      log,
		me:       me,
		receipts: receipts,
		executor: executor,
	}

	con, err := net.Register(engine.ExecutionExecution, &e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

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

// process processes events for the execution engine on the execution node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *executor.ExecutableBlock:
		return e.onExecutableBlock(ev)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onExecutableBlock is triggered when this engine receives a new block.
//
// This function passes the executable block to the block executor and
// then submits the result to the receipts engine.
func (e *Engine) onExecutableBlock(block *executor.ExecutableBlock) error {
	result, err := e.executor.ExecuteBlock(block)
	if err != nil {
		return fmt.Errorf("failed to execute block: %w", err)
	}

	// submit execution result to receipt engine
	e.receipts.SubmitLocal(result)

	return nil
}
