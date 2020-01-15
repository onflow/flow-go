package receipts

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine broadcasts execution receipts to nodes in the network.
type Engine struct {
	unit  *engine.Unit
	log   zerolog.Logger
	con   network.Conduit
	state protocol.State
	me    module.Local
}

func New(logger zerolog.Logger, net module.Network, state protocol.State, me module.Local) (*Engine, error) {
	eng := Engine{
		unit:  engine.NewUnit(),
		log:   logger,
		state: state,
		me:    me,
	}

	con, err := net.Register(engine.ReceiptProvider, &eng)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	eng.con = con

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

func (e *Engine) Wait() {
	e.unit.Wait()
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
		case *flow.ExecutionResult:
			err = e.onExecutionResult(originID, v)
		default:
			err = errors.Errorf("invalid event type (%T)", event)
		}
		if err != nil {
			return errors.Wrap(err, "could not process event")
		}
		return nil
	})
}

func (e *Engine) onExecutionResult(originID flow.Identifier, result *flow.ExecutionResult) error {
	e.log.Debug().
		Hex("block_id", result.BlockID[:]).
		Hex("result_id", logging.ID(result.ID())).
		Msg("received execution result")

	if originID != e.me.NodeID() {
		return fmt.Errorf("invalid remote request to submit execution result [%x]", result.ID())
	}

	receipt := &flow.ExecutionReceipt{
		ExecutionResult: *result,
		// TODO: include SPoCKs
		Spocks: nil,
		// TODO: sign execution receipt
		ExecutorSignature: nil,
	}

	err := e.broadcastExecutionReceipt(receipt)
	if err != nil {
		return fmt.Errorf("could not broadcast receipt: %w", err)
	}

	return nil
}

func (e *Engine) broadcastExecutionReceipt(receipt *flow.ExecutionReceipt) error {
	identities, err := e.state.Final().Identities(identity.HasRole(flow.RoleConsensus, flow.RoleVerification))
	if err != nil {
		return fmt.Errorf("could not get consensus and verification identities: %w", err)
	}

	err = e.con.Submit(receipt, identities.NodeIDs()...)
	if err != nil {
		return fmt.Errorf("could not submit execution receipts: %w", err)
	}

	return nil
}
