package computation

import (
	"context"
	"fmt"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type ComputationManager interface {
	ExecuteScript([]byte, [][]byte, *flow.Header, *delta.View) ([]byte, error)
	ComputeBlock(
		ctx context.Context,
		block *entity.ExecutableBlock,
		view *delta.View,
	) (*execution.ComputationResult, error)
	GetAccount(addr flow.Address, header *flow.Header, view *delta.View) (*flow.Account, error)
}

// Manager manages computation and execution
type Manager struct {
	log           zerolog.Logger
	me            module.Local
	protoState    protocol.State
	vm            virtualmachine.VirtualMachine
	blockComputer computer.BlockComputer
	blocks        storage.Blocks
}

func New(
	logger zerolog.Logger,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	me module.Local,
	protoState protocol.State,
	vm virtualmachine.VirtualMachine,
	blocks storage.Blocks,
) *Manager {
	log := logger.With().Str("engine", "computation").Logger()

	e := Manager{
		log:           log,
		me:            me,
		protoState:    protoState,
		vm:            vm,
		blockComputer: computer.NewBlockComputer(vm, blocks, metrics, tracer),
		blocks:        blocks,
	}

	return &e
}

func (e *Manager) ExecuteScript(script []byte, arguments [][]byte, blockHeader *flow.Header, view *delta.View) ([]byte, error) {

	result, err := e.vm.NewBlockContext(blockHeader, e.blocks).ExecuteScript(view, script, arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if !result.Succeeded() {
		return nil, fmt.Errorf("failed to execute script at block (%s): %s", blockHeader.ID(), result.Error.ErrorMessage())
	}

	encodedValue, err := jsoncdc.Encode(result.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	return encodedValue, nil
}

func (e *Manager) ComputeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	view *delta.View,
) (*execution.ComputationResult, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	result, err := e.blockComputer.ExecuteBlock(ctx, block, view)
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

func (e *Manager) GetAccount(addr flow.Address, blockHeader *flow.Header, view *delta.View) (*flow.Account, error) {
	account, err := e.vm.NewBlockContext(blockHeader, e.blocks).GetAccount(view, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounot at block (%s): %w", blockHeader.ID(), err)
	}

	return account, nil
}
