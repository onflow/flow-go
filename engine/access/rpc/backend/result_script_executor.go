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

// TODO(Uliana): add godoc to whole file
type ResultScriptExecutor struct {
	*baseScriptExecutor

	// executionStateCache provides information access to execution state snapshots for querying data at specific ExecutionResults
	executionStateCache optimistic_sync.ExecutionStateCache
}

var _ execution.ResultScriptExecutor = (*ResultScriptExecutor)(nil)

func NewResultScriptExecutor(
	log zerolog.Logger,
	minHeight, maxHeight uint64,
	executionStateCache optimistic_sync.ExecutionStateCache,
) *ResultScriptExecutor {
	logger := log.With().Str("component", "result-script-executor").Logger()

	return &ResultScriptExecutor{
		baseScriptExecutor:  NewBaseScriptExecutor(logger, minHeight, maxHeight),
		executionStateCache: executionStateCache,
	}
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
	executionResultID flow.Identifier,
) ([]byte, error) {
	registers, err := s.registers(executionResultID)
	if err != nil {
		return nil, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height, registers)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	registers, err := s.registers(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountAtBlockHeight(ctx, address, height, registers)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	registers, err := s.registers(flow.ZeroID)
	if err != nil {
		return 0, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountBalance(ctx, address, height, registers)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	registers, err := s.registers(flow.ZeroID)
	if err != nil {
		return 0, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountAvailableBalance(ctx, address, height, registers)
}

// GetAccountKeys returns a public key of Flow account by the provided address, block height and index.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	registers, err := s.registers(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKeys(ctx, address, height, registers)
}

// GetAccountKey returns
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	registers, err := s.registers(flow.ZeroID)
	if err != nil {
		return nil, err
	}

	if err := s.checkHeight(registers, height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKey(ctx, address, keyIndex, height, registers)
}

// checkHeight checks if the provided block height is within the range of indexed heights
// and compatible with the node's version.
//
// It performs several checks:
// 1. Ensures the IndexerScriptExecutor is initialized.
// 2. Compares the provided height with the highest and lowest indexed heights.
// 3. Ensures the height is within the compatible version range if version control is enabled.
//
// Parameters:
// - height: the block height to check.
//
// Returns:
// - error: if the block height is not within the indexed range or not compatible with the node's version.
//
// Expected errors:
// - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
// or if the height is before the lowest indexed height.
// - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ResultScriptExecutor) checkHeight(registers storage.RegisterIndexReader, height uint64) error {
	if height > registers.LatestHeight() {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	if height < registers.FirstHeight() {
		return fmt.Errorf("%w: block is before lowest indexed height", storage.ErrHeightNotIndexed)
	}

	if height > s.maxCompatibleHeight.Load() || height < s.minCompatibleHeight.Load() {
		return ErrIncompatibleNodeVersion
	}

	// Version control feature could be disabled. In such a case, ignore related functionality.
	if s.versionControl != nil {
		compatible, err := s.versionControl.CompatibleAtBlock(height)
		if err != nil {
			return fmt.Errorf("failed to check compatibility with block height %d: %w", height, err)
		}

		if !compatible {
			return ErrIncompatibleNodeVersion
		}
	}

	return nil
}

func (s *ResultScriptExecutor) registers(executionResultID flow.Identifier) (storage.RegisterIndexReader, error) {
	//TODO(Data availability): temp fix : remove this after GetAccount* will be updated
	if executionResultID == flow.ZeroID {
		panic("not implemented yet")
	}

	snapshot, err := s.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot for execution result %s: %w", executionResultID, err)
	}

	return snapshot.Registers(), nil
}
