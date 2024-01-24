package scripts

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
)

type Engine struct {
	unit          *engine.Unit
	log           zerolog.Logger
	queryExecutor query.Executor
	execState     state.ScriptExecutionState
}

var _ execution.ScriptExecutor = (*Engine)(nil)

func New(
	logger zerolog.Logger,
	queryExecutor query.Executor,
	execState state.ScriptExecutionState,
) *Engine {
	return &Engine{
		unit:          engine.NewUnit(),
		log:           logger.With().Str("engine", "scripts").Logger(),
		execState:     execState,
		queryExecutor: queryExecutor,
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

	blockSnapshot, header, err := e.execState.CreateStorageSnapshot(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage snapshot: %w", err)
	}

	return e.queryExecutor.ExecuteScript(
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

	blockSnapshot, _, err := e.execState.CreateStorageSnapshot(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage snapshot: %w", err)
	}

	id := flow.NewRegisterID(flow.BytesToAddress(owner), string(key))
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
	blockSnapshot, header, err := e.execState.CreateStorageSnapshot(blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage snapshot: %w", err)
	}

	return e.queryExecutor.GetAccount(ctx, addr, header, blockSnapshot)
}
