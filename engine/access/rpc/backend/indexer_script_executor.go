package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
)

// ErrIncompatibleNodeVersion indicates that node version is incompatible with the block version
var ErrIncompatibleNodeVersion = errors.New("node version is incompatible with data for block")

type IndexerScriptExecutor struct {
	*baseScriptExecutor

	// initialized signals that the index and executor are ready
	initialized *atomic.Bool

	// indexReporter provides information about the current state of the execution state indexer.
	indexReporter state_synchronization.IndexReporter
	registers     storage.RegisterIndex
}

var _ execution.IndexerScriptExecutor = (*IndexerScriptExecutor)(nil)

func NewIndexerScriptExecutor(log zerolog.Logger, registers storage.RegisterIndex, minHeight, maxHeight uint64) *IndexerScriptExecutor {
	logger := log.With().Str("component", "script-executor").Logger()

	return &IndexerScriptExecutor{
		baseScriptExecutor: NewBaseScriptExecutor(logger, minHeight, maxHeight),
		initialized:        atomic.NewBool(false),
		registers:          registers,
	}
}

// Initialize initializes the indexReporter and script executor
// This method can be called at any time after the IndexerScriptExecutor object is created. Any requests
// made to the other methods will return storage.ErrHeightNotIndexed until this method is called.
func (s *IndexerScriptExecutor) Initialize(
	indexReporter state_synchronization.IndexReporter,
	scripts *execution.Scripts,
	versionControl *version.VersionControl,
) error {
	if s.initialized.CompareAndSwap(false, true) {
		s.log.Info().Msg("script executor initialized")
		s.indexReporter = indexReporter
		s.scripts = scripts
		s.versionControl = versionControl
		return nil
	}
	return fmt.Errorf("script executor already initialized")
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
) ([]byte, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height, s.registers)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountAtBlockHeight(ctx, address, height, s.registers)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	if err := s.checkHeight(height); err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountBalance(ctx, address, height, s.registers)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	if err := s.checkHeight(height); err != nil {
		return 0, err
	}

	return s.baseScriptExecutor.GetAccountAvailableBalance(ctx, address, height, s.registers)
}

// GetAccountKeys returns a public key of Flow account by the provided address, block height and index.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKeys(ctx, address, height, s.registers)
}

// GetAccountKey returns
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the IndexerScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *IndexerScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.baseScriptExecutor.GetAccountKey(ctx, address, keyIndex, height, s.registers)
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
func (s *IndexerScriptExecutor) checkHeight(height uint64) error {
	if !s.initialized.Load() {
		return fmt.Errorf("%w: script executor not initialized", storage.ErrHeightNotIndexed)
	}

	highestHeight, err := s.indexReporter.HighestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get highest indexed height: %w", err)
	}
	if height > highestHeight {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	lowestHeight, err := s.indexReporter.LowestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get lowest indexed height: %w", err)
	}

	if height < lowestHeight {
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
