package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/storage"
)

type ScriptExecutor struct {
	*baseScriptExecutor

	// initialized signals that the index and executor are ready
	initialized *atomic.Bool

	registerSnapshot storage.RegisterSnapshotReader
}

var _ execution.ScriptExecutor = (*ScriptExecutor)(nil)

func NewScriptExecutor(log zerolog.Logger, registerSnapshot storage.RegisterSnapshotReader, minHeight, maxHeight uint64) *ScriptExecutor {
	logger := log.With().Str("component", "script-executor").Logger()

	return &ScriptExecutor{
		baseScriptExecutor: NewBaseScriptExecutor(logger, minHeight, maxHeight),
		initialized:        atomic.NewBool(false),
		registerSnapshot:   registerSnapshot,
	}
}

//TODO(Uliana): refactoring: move creating scripts from execution data indexer component and remove Initialize for ScriptExecutor cause
// it does not depend on indexer anymore

// Initialize initializes script executor.
// This method can be called at any time after the ScriptExecutor object is created. Any requests
// made to the other methods will return storage.ErrHeightNotIndexed until this method is called.
func (s *ScriptExecutor) Initialize(
	scripts *execution.Scripts,
	versionControl *version.VersionControl,
) error {
	if s.initialized.CompareAndSwap(false, true) {
		s.log.Info().Msg("script executor initialized")
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
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
) ([]byte, error) {
	return s.baseScriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height, s.registerSnapshot)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	return s.baseScriptExecutor.GetAccountAtBlockHeight(ctx, address, height, s.registerSnapshot)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	return s.baseScriptExecutor.GetAccountBalance(ctx, address, height, s.registerSnapshot)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	return s.baseScriptExecutor.GetAccountAvailableBalance(ctx, address, height, s.registerSnapshot)
}

// GetAccountKeys returns a public key of Flow account by the provided address, block height and index.
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	return s.baseScriptExecutor.GetAccountKeys(ctx, address, height, s.registerSnapshot)
}

// GetAccountKey returns
// Expected errors:
//   - Script execution related errors
//   - storage.ErrHeightNotIndexed if the ScriptExecutor is not initialized, or if the height is not indexed yet,
//     or if the height is before the lowest indexed height.
//   - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (s *ScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64) (*flow.AccountPublicKey, error) {
	return s.baseScriptExecutor.GetAccountKey(ctx, address, keyIndex, height, s.registerSnapshot)
}
