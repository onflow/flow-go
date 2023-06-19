package scripts

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	state              protocol.State
	computationManager computation.ComputationManager
	execState          state.ExecutionState
}

var _ execution.ScriptExecutor = (*Engine)(nil)

func New(
	logger zerolog.Logger,
	state protocol.State,
	computationManager computation.ComputationManager,
	execState state.ExecutionState,
) *Engine {
	return &Engine{
		unit:               engine.NewUnit(),
		log:                logger.With().Str("engine", "scripts").Logger(),
		state:              state,
		execState:          execState,
		computationManager: computationManager,
	}
}

func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) ExecuteScriptAtBlockID(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	blockID flow.Identifier,
) ([]byte, error) {

	stateCommit, err := e.execState.StateCommitmentByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}

	// return early if state with the given state commitment is not in memory
	// and already purged. This reduces allocations for scripts targeting old blocks.
	if !e.execState.HasState(stateCommit) {
		return nil, fmt.Errorf("failed to execute script at block (%s): state commitment not found (%s). this error usually happens if the reference block for this script is not set to a recent block", blockID.String(), hex.EncodeToString(stateCommit[:]))
	}

	header, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get header (%s): %w", blockID, err)
	}

	blockSnapshot := e.execState.NewStorageSnapshot(stateCommit)

	return e.computationManager.ExecuteScript(
		ctx,
		script,
		arguments,
		header,
		blockSnapshot)
}

func (e *Engine) GetRegisterAtBlockID(
	ctx context.Context,
	owner, key []byte,
	blockID flow.Identifier,
) ([]byte, error) {

	stateCommit, err := e.execState.StateCommitmentByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}

	blockSnapshot := e.execState.NewStorageSnapshot(stateCommit)

	id := flow.NewRegisterID(string(owner), string(key))
	data, err := blockSnapshot.Get(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get the register (%s): %w", id, err)
	}

	return data, nil
}

func (e *Engine) GetAccount(
	ctx context.Context,
	addr flow.Address,
	blockID flow.Identifier,
) (*flow.Account, error) {
	stateCommit, err := e.execState.StateCommitmentByBlockID(ctx, blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to get state commitment for block (%s): %w", blockID, err)
	}

	// return early if state with the given state commitment is not in memory
	// and already purged. This reduces allocations for get accounts targeting old blocks.
	if !e.execState.HasState(stateCommit) {
		return nil, fmt.Errorf(
			"failed to get account at block (%s): state commitment not "+
				"found (%s). this error usually happens if the reference "+
				"block for this script is not set to a recent block.",
			blockID.String(),
			hex.EncodeToString(stateCommit[:]))
	}

	block, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	blockSnapshot := e.execState.NewStorageSnapshot(stateCommit)

	return e.computationManager.GetAccount(ctx, addr, block, blockSnapshot)
}
