package provider

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// An Engine provides means of accessing data about execution state. Also broadcasts execution receipts to nodes in the network and
type Engine struct {
	unit       *engine.Unit
	log        zerolog.Logger
	receiptCon network.Conduit
	state      protocol.State
	me         module.Local
	execStateCon network.Conduit
}

func New(logger zerolog.Logger, net module.Network, state protocol.State, me module.Local, ) (*Engine, error) {
	log := logger.With().Str("engine", "receipts").Logger()

	eng := Engine{
		unit:  engine.NewUnit(),
		log:   log,
		state: state,
		me:    me,
	}

	receiptCon, err := net.Register(engine.ReceiptProvider, &eng)
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
		case *flow.ExecutionResult:
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

func (e *Engine) onExecutionResult(originID flow.Identifier, result *flow.ExecutionResult) error {
	e.log.Debug().
		Hex("block_id", logging.ID(result.BlockID)).
		Hex("result_id", logging.Entity(result)).
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

func (e *Engine) onExecutionStateRequest(originID flow.Identifier, req *messages.ExecutionStateRequest) error {
	//chunkID := req.ChunkID
	//
	//e.log.Info().
	//	Hex("origin_id", logging.ID(originID)).
	//	Hex("chunk_id", logging.ID(chunkID)).
	//	Msg("received execution state request")
	//
	//id, err := e.state.Final().Identity(originID)
	//if err != nil {
	//	return fmt.Errorf("invalid origin id (%s): %w", id, err)
	//}
	//
	//if id.Role != flow.RoleVerification {
	//	return fmt.Errorf("invalid role for requesting execution state: %s", id.Role)
	//}
	//
	//registers, err := e.execState.GetChunkRegisters(chunkID)
	//if err != nil {
	//	return fmt.Errorf("could not retrieve chunk state (id=%s): %w", chunkID, err)
	//}
	//
	//msg := &messages.ExecutionStateResponse{State: flow.ChunkState{
	//	ChunkID:   chunkID,
	//	Registers: registers,
	//}}
	//
	//err = e.execStateCon.Submit(msg, id.NodeID)
	//if err != nil {
	//	return fmt.Errorf("could not submit response for chunk state (id=%s): %w", chunkID, err)
	//}

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
