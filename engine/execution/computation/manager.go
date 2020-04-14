package computation

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/cadence"
	encoding "github.com/dapperlabs/cadence/encoding/xdr"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ComputationManager interface {
	ExecuteScript([]byte, *flow.Header, *delta.View) ([]byte, error)
	ComputeBlock(block *entity.ExecutableBlock, view *delta.View) (*execution.ComputationResult, error)
	GetAccount(address flow.Address, blockHeader *flow.Header, view *delta.View) (*flow.Account, error)
}

// Manager manages computation and execution
type Manager struct {
	log           zerolog.Logger
	me            module.Local
	protoState    protocol.State
	vm            virtualmachine.VirtualMachine
	blockComputer computer.BlockComputer
}

func New(
	logger zerolog.Logger,
	me module.Local,
	protoState protocol.State,
	vm virtualmachine.VirtualMachine,
) *Manager {
	log := logger.With().Str("engine", "computation").Logger()

	e := Manager{
		log:           log,
		me:            me,
		protoState:    protoState,
		vm:            vm,
		blockComputer: computer.NewBlockComputer(vm),
	}

	return &e
}

func (e *Manager) ExecuteScript(script []byte, blockHeader *flow.Header, view *delta.View) ([]byte, error) {

	result, err := e.vm.NewBlockContext(blockHeader).ExecuteScript(view, script)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if !result.Succeeded() {
		return nil, fmt.Errorf("failed to execute script at block (%s): %w", blockHeader.ID(), result.Error)
	}

	value := cadence.ConvertValue(result.Value)

	encodedValue, err := encoding.Encode(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	return encodedValue, nil
}

func (e *Manager) ComputeBlock(block *entity.ExecutableBlock, view *delta.View) (*execution.ComputationResult, error) {
	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	result, err := e.blockComputer.ExecuteBlock(block, view)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return nil, fmt.Errorf("failed to execute block: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock.Block)).
		Msg("computed block result")

	return result, nil
}

func (e *Manager) GetAccount(address flow.Address, blockHeader *flow.Header, view *delta.View) (*flow.Account, error) {

	result := e.vm.NewBlockContext(blockHeader).GetAccount(view, address)
	return result, nil
}
