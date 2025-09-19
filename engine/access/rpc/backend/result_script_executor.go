package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/storage"
)

// TODO(Uliana): rename and add godoc to whole file
type ResultBasedScriptExecutor struct {
	*baseScriptExecutor

	// executionStateCache provides information access to execution state snapshots for querying data at specific ExecutionResults
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ execution.ResultBasedScriptExecutor = (*ResultBasedScriptExecutor)(nil)

func NewResultBasedScriptExecutor(
	log zerolog.Logger,
	minHeight, maxHeight uint64,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *ResultBasedScriptExecutor {
	logger := log.With().Str("component", "result-based-script-executor").Logger()

	return &ResultBasedScriptExecutor{
		baseScriptExecutor:  NewBaseScriptExecutor(logger, minHeight, maxHeight),
		executionStateCache: executionStateCache,
	}
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
	executionResultID flow.Identifier,
) ([]byte, error) {
	registerSnapshot, err := s.registerSnapshot(executionResultID)
	if err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height, registerSnapshot)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	registerSnapshot, err := s.registerSnapshot(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountAtBlockHeight(ctx, address, height, registerSnapshot)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	registerSnapshot, err := s.registerSnapshot(flow.ZeroID)
	if err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountBalance(ctx, address, height, registerSnapshot)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	registerSnapshot, err := s.registerSnapshot(flow.ZeroID)
	if err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountAvailableBalance(ctx, address, height, registerSnapshot)
}

// GetAccountKeys returns a public key of Flow account by the provided address, block height and index.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	registerSnapshot, err := s.registerSnapshot(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKeys(ctx, address, height, registerSnapshot)
}

// GetAccountKey returns
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultBasedScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	registers, err := s.registerSnapshot(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKey(ctx, address, keyIndex, height, registers)
}

func (s *ResultBasedScriptExecutor) registerSnapshot(executionResultID flow.Identifier) (storage.RegisterSnapshotReader, error) {
	//TODO(Data availability): temp fix : remove this after GetAccount* will provide executionResultID
	if executionResultID == flow.ZeroID {
		panic("not implemented yet")
	}

	snapshot, err := s.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	return snapshot.Registers(), nil
}
