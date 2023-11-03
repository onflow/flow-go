package scripts

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// TODO(leo): move it.
var ErrStateCommitmentPruned = fmt.Errorf("state commitment not found")

// ScriptExecutionState is a subset of the `state.ExecutionState` interface purposed to only access the state
// used for script execution and not mutate the execution state of the blockchain.
type ScriptExecutionState interface {
	IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error)
	// NewStorageSnapshot returns a storage snapshot at the end of the given block for retrieving registers,
	// the caller needs to ensure the block is executed, otherwise BlockNotExecuted would be returned.
	NewStorageSnapshot(blockID flow.Identifier, height uint64) snapshot.StorageSnapshot
}

type Engine struct {
	unit          *engine.Unit
	log           zerolog.Logger
	state         protocol.State
	queryExecutor query.Executor
	execState     state.ScriptExecutionState
}

var _ execution.ScriptExecutor = (*Engine)(nil)

func New(
	logger zerolog.Logger,
	state protocol.State,
	queryExecutor query.Executor,
	execState state.ScriptExecutionState,
) *Engine {
	return &Engine{
		unit:          engine.NewUnit(),
		log:           logger.With().Str("engine", "scripts").Logger(),
		state:         state,
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
	blockSnapshot, header, err := e.getSnapshotAtBlockID(blockID)
	if err != nil {
		return nil, err
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

	blockSnapshot, _, err := e.getSnapshotAtBlockID(blockID)
	if err != nil {
		return nil, err
	}

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
	blockSnapshot, header, err := e.getSnapshotAtBlockID(blockID)
	if err != nil {
		return nil, err
	}

	return e.queryExecutor.GetAccount(ctx, addr, header, blockSnapshot)
}

func (e *Engine) getSnapshotAtBlockID(blockID flow.Identifier) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := e.state.AtBlockID(blockID).Head()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get block (%s): %w", blockID, err)
	}

	executed, err := e.execState.IsBlockExecuted(header.Height, blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to check if block (%s) is executed: %w", blockID, err)
	}

	if !executed {
		return nil, nil, fmt.Errorf("block (%s) is not executed", blockID)
	}

	// create a snapshot powered by register store
	blockSnapshot := e.execState.NewStorageSnapshot(blockID, header.Height)
	return blockSnapshot, header, nil
}
