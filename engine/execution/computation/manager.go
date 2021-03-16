package computation

import (
	"context"
	"fmt"

	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *fvm.Programs) error
	GetAccount(fvm.Context, flow.Address, state.View, *fvm.Programs) (*flow.Account, error)
}

type ComputationManager interface {
	ExecuteScript([]byte, [][]byte, *flow.Header, state.View) ([]byte, error)
	ComputeBlock(
		ctx context.Context,
		block *entity.ExecutableBlock,
		view state.View,
	) (*execution.ComputationResult, error)
	GetAccount(addr flow.Address, header *flow.Header, view state.View) (*flow.Account, error)
}

// Manager manages computation and execution
type Manager struct {
	log           zerolog.Logger
	me            module.Local
	protoState    protocol.State
	vm            VirtualMachine
	vmCtx         fvm.Context
	blockComputer computer.BlockComputer
	programsCache *ProgramsCache
}

func New(
	logger zerolog.Logger,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	me module.Local,
	protoState protocol.State,
	vm VirtualMachine,
	vmCtx fvm.Context,
	programsCacheSize uint,
) (*Manager, error) {
	log := logger.With().Str("engine", "computation").Logger()

	blockComputer, err := computer.NewBlockComputer(
		vm,
		vmCtx,
		metrics,
		tracer,
		log.With().Str("component", "block_computer").Logger(),
	)

	if err != nil {
		return nil, fmt.Errorf("cannot create block computer: %w", err)
	}

	programsCache, err := NewProgramsCache(programsCacheSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create programs cache: %w", err)
	}

	e := Manager{
		log:           log,
		me:            me,
		protoState:    protoState,
		vm:            vm,
		vmCtx:         vmCtx,
		blockComputer: blockComputer,
		programsCache: programsCache,
	}

	return &e, nil
}

func (e *Manager) getChildProgramsOrEmpty(blockID flow.Identifier) *fvm.Programs {
	programs := e.programsCache.Get(blockID)
	if programs == nil {
		return fvm.NewEmptyPrograms()
	}
	return programs.ChildPrograms()
}

func (e *Manager) ExecuteScript(code []byte, arguments [][]byte, blockHeader *flow.Header, view state.View) ([]byte, error) {
	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(blockHeader))

	script := fvm.Script(code).WithArguments(arguments...)

	programs := e.getChildProgramsOrEmpty(blockHeader.ID())

	err := e.vm.Run(blockCtx, script, view, programs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if script.Err != nil {
		return nil, fmt.Errorf("failed to execute script at block (%s): %s", blockHeader.ID(), script.Err.Error())
	}

	encodedValue, err := jsoncdc.Encode(script.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	return encodedValue, nil
}

func (e *Manager) ComputeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	view state.View,
) (*execution.ComputationResult, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	var programs *fvm.Programs
	fromCache := e.programsCache.Get(block.ParentID())

	if fromCache == nil {
		programs = fvm.NewEmptyPrograms()
	} else {
		programs = fromCache.ChildPrograms()
	}

	result, err := e.blockComputer.ExecuteBlock(ctx, block, view, programs)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return nil, fmt.Errorf("failed to execute block: %w", err)
	}

	toInsert := programs

	// if we have item from cache and there were no changes
	// insert it under new block, to prevent long chains
	if fromCache != nil && !programs.HasChanges() {
		toInsert = fromCache
	}

	e.programsCache.Set(block.ID(), toInsert)

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock.Block)).
		Msg("computed block result")

	return result, nil
}

func (e *Manager) GetAccount(address flow.Address, blockHeader *flow.Header, view state.View) (*flow.Account, error) {
	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(blockHeader))

	programs := e.getChildProgramsOrEmpty(blockHeader.ID())

	account, err := e.vm.GetAccount(blockCtx, address, view, programs)
	if err != nil {
		return nil, fmt.Errorf("failed to get account at block (%s): %w", blockHeader.ID(), err)
	}

	return account, nil
}
